// Copyright 2024 CloudDetail
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"sort"
	"strconv"
)

// The ServiceInstances contains all the mappings for the Pod, Container, and VM scenarios. The data without the Pod has been removed.
type ServiceInstances struct {
	InstanceMap map[string]*ServiceInstance
}

func NewServiceInstances() *ServiceInstances {
	return &ServiceInstances{
		InstanceMap: make(map[string]*ServiceInstance),
	}
}

func (instances *ServiceInstances) AddInstances(list []*ServiceInstance) {
	for _, instance := range list {
		if instance.PodName != "" {
			existInstance, ok := instances.InstanceMap[instance.getPodInstanceId()]
			if ok && instance.Pid == 1 {
				continue
			}

			if ok && existInstance.Pid == 1 {
				delete(instances.InstanceMap, existInstance.getVMInstanceId())
			}

			instances.InstanceMap[instance.getPodInstanceId()] = instance
			instances.InstanceMap[instance.getContainerInstanceId()] = instance
			instances.InstanceMap[instance.getVMInstanceId()] = instance
		} else {
			instanceId := ""
			if instance.ContainerId != "" {
				instanceId = instance.getContainerInstanceId()
			} else {
				if instance.Pid == 1 {
					continue
				}
				instanceId = instance.getVMInstanceId()
			}
			if _, exist := instances.InstanceMap[instanceId]; !exist {
				// Do not overwrite if Pod already exists
				instances.InstanceMap[instanceId] = instance
			}
		}
	}
}

func (instances *ServiceInstances) GetPodInstances() []string {
	pods := make([]string, 0)
	for instanceId, instance := range instances.InstanceMap {
		// de-weight
		if instance.PodName != "" && instanceId == instance.PodName {
			pods = append(pods, instance.PodName)
		}
	}
	return pods
}

func (instances *ServiceInstances) GetInstances() []*ServiceInstance {
	instanceList := make([]*ServiceInstance, 0)
	if len(instances.InstanceMap) == 0 {
		return instanceList
	}

	for _, instance := range instances.GetInstanceIdMap() {
		instanceList = append(instanceList, instance)
	}
	return instanceList
}

func (instances *ServiceInstances) GetInstanceIds() []string {
	instanceIdList := make([]string, 0)
	if len(instances.InstanceMap) == 0 {
		return instanceIdList
	}

	for instanceId := range instances.GetInstanceIdMap() {
		instanceIdList = append(instanceIdList, instanceId)
	}
	// sort based on name
	sort.Strings(instanceIdList)
	return instanceIdList
}

// find out the non-duplicate instance list from the collected instance information
//
// HACK when two process with the same information except one of them has pid = 1,
// we always return the instance with pid > 1
func (instances *ServiceInstances) GetInstanceIdMap() map[string]*ServiceInstance {
	instanceMap := make(map[string]*ServiceInstance)
	for _, instance := range instances.InstanceMap {
		if instance.PodName != "" {
			instanceId := instance.getPodInstanceId()
			_, find := instanceMap[instanceId]
			if !find || instance.Pid > 1 {
				instanceMap[instanceId] = instance
			}
		} else if instance.ContainerId != "" {
			instanceId := instance.getContainerInstanceId()
			_, find := instanceMap[instanceId]
			if !find || instance.Pid > 1 {
				instanceMap[instanceId] = instance
			}
		} else {
			instanceMap[instance.getVMInstanceId()] = instance
		}
	}
	return instanceMap
}

type ServiceInstance struct {
	ServiceName string `json:"service"`     // service name
	ContainerId string `json:"containerId"` // container ID
	PodName     string `json:"podName"`     // Pod name
	Namespace   string `json:"-"`
	NodeName    string `json:"nodeName"` // hostname
	Pid         int64  `json:"pid"`      // process number
	NodeIP      string `json:"nodeIp"`
	ClusterID   string `json:"clusterId"`
}

func (i *ServiceInstance) MatchSvcTags(group string, tags map[string]string) bool {
	switch group {
	case "app":
		if len(i.ServiceName) > 0 {
			return i.ServiceName == tags["svc_name"]
		}
	case "container":
		if len(i.PodName) > 0 {
			pod, find := tags["pod"]
			if !find {
				return false
			}
			namespace, find := tags["namespace"]
			if !find {
				return false
			}
			return i.PodName == pod && i.Namespace == namespace
		}
	case "network":
		if len(i.PodName) > 0 {
			pod, find := tags["src_pod"]
			if !find {
				return false
			}
			namespace, find := tags["src_namespace"]
			if !find {
				return false
			}
			return i.PodName == pod && i.Namespace == namespace
		} else if i.Pid > 0 {
			pid, find := tags["pid"]
			if !find {
				return false
			}
			node, find := tags["src_node"]
			if !find {
				return false
			}

			return strconv.FormatInt(i.Pid, 10) == pid && i.NodeName == node
		}
	case "infra":
		if len(i.NodeName) > 0 {
			node, find := tags["instance_name"]
			if !find {
				return false
			}
			return node == i.NodeName
		}
	}
	return false
}

func (instance *ServiceInstance) GetInstanceId() string {
	if instance.PodName != "" {
		return instance.getPodInstanceId()
	}
	if instance.ContainerId != "" {
		return instance.getContainerInstanceId()
	}
	return instance.getVMInstanceId()
}

func (instance *ServiceInstance) getPodInstanceId() string {
	return instance.PodName
}

func (instance *ServiceInstance) getContainerInstanceId() string {
	return fmt.Sprintf("%s@%s@%s", instance.ServiceName, instance.NodeName, instance.ContainerId)
}

func (instance *ServiceInstance) getVMInstanceId() string {
	return fmt.Sprintf("%s@%s@%d", instance.ServiceName, instance.NodeName, instance.Pid)
}
