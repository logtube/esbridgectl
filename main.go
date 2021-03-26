package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/olivere/elastic/v7"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"strings"
	"time"
)

var (
	accessMode int32 = 0644
)

const (
	taskLabelKey   = "managed-by.logtube"
	taskLabelValue = "esbridgectl"
)

var (
	taskSelector = fmt.Sprintf("%s=%s", taskLabelKey, taskLabelValue)
)

func main() {
	var err error
	defer func(err *error) {
		if *err != nil {
			log.Println("exited with error:", (*err).Error())
			os.Exit(1)
		} else {
			log.Println("exited")
		}
	}(&err)

	var (
		optDryRun         bool
		optESURL          string
		optKubeconfig     string
		optNamespace      string
		optTasks          int
		optDays           int
		optConfigMap      string
		optStorageClass   string
		optStorageRequest string
		optImage          string
	)

	flag.BoolVar(&optDryRun, "dry-run", false, "dry run")
	flag.StringVar(&optImage, "image", "guoyk/esbridge", "container image")
	flag.StringVar(&optESURL, "es-url", "http://127.0.0.1:9200", "elasticsearch url")
	flag.StringVar(&optKubeconfig, "kubeconfig", "kubeconfig", "kubeconfig file")
	flag.StringVar(&optNamespace, "namespace", "esmaint", "namespace in kubernetes cluster")
	flag.IntVar(&optTasks, "tasks", 4, "maximum concurrent tasks")
	flag.IntVar(&optDays, "days", 95, "keep days of indices")
	flag.StringVar(&optConfigMap, "config-map", "esbridge-cfg", "name of the configmap to feed esbridge")
	flag.StringVar(&optStorageClass, "storage-class", "local-path", "storage class of pvc")
	flag.StringVar(&optStorageRequest, "storage-request", "200Gi", "storage request for pvc")
	flag.Parse()

	var candidateIndices []string
	{
		midnight := dateMidnight(time.Now())

		var client *elastic.Client
		if client, err = elastic.NewClient(elastic.SetURL(optESURL), elastic.SetSniff(false)); err != nil {
			return
		}

		var resp elastic.CatIndicesResponse
		if resp, err = client.CatIndices().Do(context.Background()); err != nil {
			return
		}

		for _, row := range resp {
			var t time.Time
			var ok bool
			if t, ok = dateFromIndex(row.Index); !ok {
				continue
			}
			if midnight.Sub(t)/(time.Hour*24) >= time.Duration(optDays) {
				log.Println("Candidate:", row.Index)
				candidateIndices = append(candidateIndices, row.Index)
			}
		}

		sortCandidateIndices(candidateIndices)
	}

	var config *rest.Config
	if config, err = clientcmd.BuildConfigFromFlags("", optKubeconfig); err != nil {
		return
	}

	var klient *kubernetes.Clientset
	if klient, err = kubernetes.NewForConfig(config); err != nil {
		return
	}

	// delete completed Job

	var jobList *batchv1.JobList
	if jobList, err = klient.BatchV1().Jobs(optNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: taskSelector,
	}); err != nil {
		return
	}

	jobCount := len(jobList.Items)

	for _, job := range jobList.Items {
		var done bool
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				done = true
				log.Println("Saw Complete", job.Name)
			}
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				done = true
				log.Println("Saw Failed", job.Name)
			}
		}
		if !done {
			continue
		}

		log.Println("Delete Job", job.Name)
		if !optDryRun {
			_ = klient.BatchV1().Jobs(optNamespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{})
		}

		log.Println("Delete PVC", job.Name)
		if !optDryRun {
			_ = klient.CoreV1().PersistentVolumeClaims(optNamespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{})
		}

		jobCount--
	}

	slots := optTasks - jobCount
	if slots < 0 {
		slots = 0
	}
	log.Println("Remaining Slots:", slots)

	if slots == 0 {
		return
	}

	if slots > len(candidateIndices) {
		candidateIndices = candidateIndices[0:slots]
	}

	log.Println("Indices:", strings.Join(candidateIndices, ", "))

	for _, index := range candidateIndices {
		taskName := "task-" + index

		pvc := &corev1.PersistentVolumeClaim{}
		pvc.Namespace = optNamespace
		pvc.Name = taskName
		pvc.Labels = map[string]string{
			taskLabelKey: taskLabelValue,
		}
		pvc.Spec.AccessModes = append(pvc.Spec.AccessModes, corev1.ReadWriteOnce)
		pvc.Spec.StorageClassName = &optStorageClass
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(optStorageRequest),
		}

		log.Printf("Create PVC: %+v", pvc)
		if !optDryRun {
			if _, err = klient.CoreV1().PersistentVolumeClaims(optNamespace).Create(context.Background(), pvc, metav1.CreateOptions{}); err != nil {
				return
			}
		}

		job := &batchv1.Job{}
		job.Namespace = optNamespace
		job.Name = taskName
		job.Labels = map[string]string{
			taskLabelKey: taskLabelValue,
		}
		job.Spec.Template.Labels = map[string]string{
			"k8s-app": taskName,
		}
		job.Spec.Template.Annotations = map[string]string{
			"tke.cloud.tencent.com/vpc-ip-claim-delete-policy": "Immediate",
		}
		spec := corev1.PodSpec{}

		container := corev1.Container{}

		container.Image = "guoyk/esbridge"
		container.ImagePullPolicy = corev1.PullAlways
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "ESBRIDGE_INDEX",
			Value: index,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "ESBRIDGE_BATCH_SIZE",
			Value: "2000",
		})
		container.Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2000Mi"),
		}
		container.Resources.Limits = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("6000Mi"),
		}

		spec.Containers = []corev1.Container{container}
		spec.RestartPolicy = corev1.RestartPolicyOnFailure

		volCfg := corev1.Volume{}
		volCfg.Name = "vol-cfg"
		volCfg.ConfigMap = &corev1.ConfigMapVolumeSource{}
		volCfg.ConfigMap.Name = optConfigMap
		volCfg.ConfigMap.DefaultMode = &accessMode

		volData := corev1.Volume{}
		volData.Name = "vol-data"
		volData.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{}
		volData.PersistentVolumeClaim.ClaimName = taskName

		spec.Volumes = []corev1.Volume{volCfg, volData}

		job.Spec.Template.Spec = spec

		log.Printf("Create Job: %+v", job)
		if !optDryRun {
			if _, err = klient.BatchV1().Jobs(optNamespace).Create(context.Background(), job, metav1.CreateOptions{}); err != nil {
				return
			}
		}

		if !optDryRun {
			time.Sleep(time.Second * 10)

			if pvc, err = klient.CoreV1().PersistentVolumeClaims(optNamespace).Get(context.Background(), taskName, metav1.GetOptions{}); err != nil {
				return
			}

			if pvc.Spec.VolumeName == "" {
				err = fmt.Errorf("failed to locate pv name for pvc: %s", taskName)
				return
			}

			log.Println("PV:", pvc.Spec.VolumeName)

			var pv *corev1.PersistentVolume
			if pv, err = klient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{}); err != nil {
				return
			}

			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete

			log.Println("PV Patch:", pvc.Spec.VolumeName)
			if _, err = klient.CoreV1().PersistentVolumes().Update(context.Background(), pv, metav1.UpdateOptions{}); err != nil {
				return
			}
		}
	}

}