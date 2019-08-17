/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"github.com/mattbaird/jsonpatch"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"strings"
)

var (
	envVars []corev1.EnvVar
)


func shouldMutate(pod corev1.Pod, namespace string) bool {
	klog.V(2).Infof("ShouldMuate...")
	//check if the pod's status includes the injected annotation
	annotations := pod.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	status := annotations[admissionWebhookAnnotationStatusKey]

	if strings.ToLower(status) == "injected" {
		//already injected, nothing left to do
		klog.V(2).Info("Already injected")
		return false;
	} else {
		switch strings.ToLower(annotations[admissionWebhookAnnotationInjectKey]) {
		case "y", "yes", "true", "on":
			//pod-level annotation trumps all else
			klog.V(2).Info("On at pod level, returning")
			return true
		case "n", "no", "false", "off":
			klog.V(2).Info("Off at pod level, returning")
			return false;
		}
	}
	klog.V(2).Infof("Nothing at the pod level, grabbing namespace - %v", namespace)
	// at this point it hasn't already been injected, and there's no preference at the pod level, so check the namespace
	opts := metav1.GetOptions{}
	ns, err := clientset.CoreV1().Namespaces().Get(namespace, opts)
	if err != nil {
		klog.Errorf("Unable to get namespace: %v", err)
		return false
	}
	klog.V(2).Infof("Got namespace: %v", ns)

	nsLabels := ns.ObjectMeta.Labels
	if nsLabels == nil {
		nsLabels = map[string]string{}
	}

	klog.V(2).Infof("Current Namespace annotations: %v", nsLabels)

	switch strings.ToLower(nsLabels[admissionWebhookAnnotationInjectKey]) {
	case "y", "yes", "true", "on":
		klog.V(2).Info("on at namespace level")
		return true
	default:
		klog.V(2).Info("Not on at namespace level")
		return false;
	}



}

func mutatePods(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	klog.Info("mutating pods")
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = false


	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		klog.Errorf("expect resource to be %s", podResource)
		return &reviewResponse
	}

	raw := ar.Request.Object.Raw
	pod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()

	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		klog.Error(err)
		return toAdmissionResponse(err)
	}

	klog.V(2).Infof("original pod: %v", pod)

	if !shouldMutate(pod, ar.Request.Namespace) {
		klog.V(2).Info("Should Mutate == false")
		reviewResponse.Allowed = true
		return &reviewResponse
	} else {
		klog.V(2).Info("Should Mutate == true")
	}

	err := configEnvVars()
	if err != nil {
		klog.Error("Unable to load env vars from config file")
		return &reviewResponse
	}

	for idx, ic := range pod.Spec.InitContainers {
		toBeIgnored := make(map[string]bool)
		for _, ev := range ic.Env {
			_, exists := envVarNamesLower[strings.ToLower(ev.Name)]
			if exists {
				toBeIgnored[strings.ToLower(ev.Name)] = true
			}
		}
		for _, ev := range envVars {
			_, exists := toBeIgnored[strings.ToLower(ev.Name)]
			if !exists {
				klog.V(2).Infof("Appending env var: %v/%v", ev.Name, ev.Value)
				if pod.Spec.InitContainers[idx].Env == nil {
					pod.Spec.InitContainers[idx].Env = []corev1.EnvVar{}
				}
				pod.Spec.InitContainers[idx].Env = append(pod.Spec.InitContainers[idx].Env, ev)
			}
		}
	}

	for idx, c := range pod.Spec.Containers {
		toBeIgnored := make(map[string]bool)
		for _, ev := range c.Env {
			_, exists := envVarNamesLower[strings.ToLower(ev.Name)]
			if exists {
				toBeIgnored[strings.ToLower(ev.Name)] = true
			}
		}
		for _, ev := range envVars {
			klog.V(2).Infof("looking at %v", ev)
			klog.V(2).Infof("envnameslower: %v", toBeIgnored)
			_, exists := toBeIgnored[strings.ToLower(ev.Name)]
			klog.V(2).Infof("exists: %v", exists)
			if !exists {
				klog.V(2).Infof("Appending env var: %v/%v to container: %v", ev.Name, ev.Value, c)
				if pod.Spec.Containers[idx].Env == nil {
					pod.Spec.Containers[idx].Env = []corev1.EnvVar{}
				}
				pod.Spec.Containers[idx].Env = append(pod.Spec.Containers[idx].Env, ev)
				klog.V(2).Infof("Container env right after appending env: %v", c)
				klog.V(2).Infof("reference from pod: %v", pod.Spec.Containers[idx])
			}
		}

	}

	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = make(map[string]string)
	}
	annotations := pod.ObjectMeta.Annotations
	annotations[admissionWebhookAnnotationStatusKey] = "injected"
	pod.ObjectMeta.Annotations = annotations


	newPodRaw, err := json.Marshal(pod)


	if err != nil {
		klog.Errorf("unable to serialize mutated pod to json")
		return &reviewResponse
	}

	patch, err := jsonpatch.CreatePatch(raw, newPodRaw)
	if err != nil {
		klog.Errorf("unable to create json patch")
		return &reviewResponse
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		klog.Errorf("unable to marshal patch bytes")
		return &reviewResponse
	}


	klog.V(2).Infof("JSON PATCH : %v", string(patchBytes))

	reviewResponse.Allowed = true
	reviewResponse.Patch = patchBytes
	//pt := v1beta1.PatchTypeJSONPatch
	reviewResponse.PatchType = func() *v1beta1.PatchType {
		pt := v1beta1.PatchTypeJSONPatch
		return &pt
	}()

	return &reviewResponse
}


