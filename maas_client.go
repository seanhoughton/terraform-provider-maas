package main

import (
	"log"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/juju/gomaasapi"
)

// maasListAllNodes This is a *low level* function that access a MAAS Server and returns an array of MAASObject
// The function takes a pointer to an already active MAASObject and returns a JSONObject array and an error code
func maasListAllNodes(maas *gomaasapi.MAASObject) ([]gomaasapi.JSONObject, error) {
	nodeListing := maas.GetSubObject("machines")
	log.Println("[DEBUG] [maasListAllNodes] Fetching list of nodes...")
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		log.Println("[ERROR] [maasListAllNodes] Unable to get list of nodes ...")
		return nil, err
	}

	listNodes, err := listNodeObjects.GetArray()
	if err != nil {
		log.Println("[ERROR] [maasListAllNodes] Unable to get the node list array ...")
		return nil, err
	}
	return listNodes, err
}

// maasGetSingleNodeByID
// This is a *low level* function that access a MAAS Server and returns a MAASObject referring to a single MAAS managed node.
// The function takes a pointer to an already active MAASObject as well as a system_id and returns a MAASObject array and an error code.
func maasGetSingleNodeByID(maas *gomaasapi.MAASObject, system_id string) (gomaasapi.MAASObject, error) {
	log.Printf("[DEBUG] [maasGetSingleNodeByID] Getting a node (%s) from MAAS\n", system_id)
	nodeObject, err := maas.GetSubObject("machines").GetSubObject(system_id).Get()
	if err != nil {
		log.Printf("[ERROR] [maasGetSingleNodeByID] Unable to get node (%s) from MAAS\n", system_id)
		return gomaasapi.MAASObject{}, err
	}
	return nodeObject, nil
}

// maasDeleteNode This is a *low level* function that deletes the provided maas node
func maasDeleteNode(maas *gomaasapi.MAASObject, system_id string) error {
	log.Printf("[DEBUG] [maasDeleteNode] Deleting node with id %s", system_id)
	err := maas.GetSubObject("machines").GetSubObject(system_id).Delete()
	if err != nil {
		log.Println("[ERROR] [maasDeleteNode] Unable to delete node ... bailing")
		return err
	}
	return nil
}

// maasAllocateNodes This is a *low level* function that attempts to acquire a MAAS managed node for future deployment.
func maasAllocateNodes(maas *gomaasapi.MAASObject, params url.Values) (gomaasapi.MAASObject, error) {
	log.Printf("[DEBUG] [maasAllocateNodes] Allocating one or more nodes with following params: %+v", params)

	nodeObject, err := maas.GetSubObject("machines").CallPost("allocate", params)
	if err != nil {
		log.Println("[ERROR] [maasAllocateNodes] Unable to acquire a node ... bailing")
		return gomaasapi.MAASObject{}, err
	}
	return nodeObject.GetMAASObject()
}

// maasReleaseNode Releases an aquired node back as a node in the ready state
func maasReleaseNode(maas *gomaasapi.MAASObject, system_id string, params url.Values) error {
	log.Printf("[DEBUG] [maasReleaseNode] Releasing node: %s", system_id)

	_, err := maas.GetSubObject("machines").GetSubObject(system_id).CallPost("release", params)
	if err != nil {
		log.Printf("[DEBUG] [maasReleaseNode] Unable to release node (%s)", system_id)
		return err
	}
	return nil
}

func getMachineStatus(controller gomaasapi.Controller, systemID string) resource.StateRefreshFunc {
	log.Printf("[DEBUG] [getNodeStatus] Getting stat of node: %s", systemID)
	return func() (interface{}, string, error) {
		machines, err := controller.Machines(gomaasapi.MachinesArgs{SystemIDs: []string{systemID}})
		if err != nil || len(machines) == 0 {
			log.Printf("[ERROR] [getNodeStatus] Unable to get node: %s\n", systemID)
			return nil, "", err
		}
		return machines[0], machines[0].StatusName(), nil
	}
}

// getSingleNodeByID Convenience function to get a NodeInfo object for a single MAAS node.
// The function takes a fully initialized MAASObject and returns a NodeInfo, error
func getSingleNodeByID(maas *gomaasapi.MAASObject, system_id string) (*NodeInfo, error) {
	log.Printf("[DEBUG] [getSingleNode] getting node (%s) information\n", system_id)
	nodeObject, err := maasGetSingleNodeByID(maas, system_id)
	if err != nil {
		log.Printf("[ERROR] [getSingleNode] Unable to get NodeInfo object for node: %s\n", system_id)
		return nil, err
	}

	return toNodeInfo(&nodeObject)
}

// getAllNodes Convenience function to get a NodeInfo slice of all of the nodes.
// The function takes a fully initialized MAASObject and returns a slice of all of the nodes.
func getAllNodes(maas *gomaasapi.MAASObject) ([]NodeInfo, error) {
	log.Println("[DEBUG] [getAllNodes] Getting all of the MAAS managed nodes' information")
	allNodes, err := maasListAllNodes(maas)
	if err != nil {
		log.Println("[ERROR] [getAllNodes] Unable to get MAAS nodes")
		return nil, err
	}

	allNodeInfo := make([]NodeInfo, 0, 10)

	for _, nodeObj := range allNodes {
		maasObject, err := nodeObj.GetMAASObject()
		if err != nil {
			log.Println("[ERROR] [getAllNodes] Unable to get MAASObject object")
			return nil, err
		}

		node, err := toNodeInfo(&maasObject)
		if err != nil {
			log.Println("[ERROR] [getAllNodes] Unable to get NodeInfo object for node")
			return nil, err
		}

		allNodeInfo = append(allNodeInfo, *node)

	}
	return allNodeInfo, err
}

// nodeDo Take an action against a specific node
func nodeDo(maas *gomaasapi.MAASObject, system_id string, action string, params url.Values) error {
	log.Printf("[DEBUG] [nodeDo] system_id: %s, action: %s, params: %+v", system_id, action, params)

	nodeObject, err := maasGetSingleNodeByID(maas, system_id)
	if err != nil {
		log.Printf("[ERROR] [nodeDo] Unable to get node (%s) information.\n", system_id)
		return err
	}

	_, err = nodeObject.CallPost(action, params)
	if err != nil {
		log.Printf("[ERROR] [nodeDo] Unable to perform action (%s) on node (%s).  Failed withh error (%s)\n", action, system_id, err)
		return err
	}
	return nil
}

// nodesAllocate Aloocate a node
func nodesAllocate(maas *gomaasapi.MAASObject, params url.Values) (*NodeInfo, error) {
	log.Println("[DEBUG] [nodesAllocate] Attempting to allocate one or more MAAS managed nodes")

	maasNodesObject, err := maasAllocateNodes(maas, params)
	if err != nil {
		log.Println("[ERROR] [nodesAllocate] Unable to allocate node ... bailing")
		return nil, err
	}

	return toNodeInfo(&maasNodesObject)
}

// nodeRelease release a node back into the ready state
func nodeRelease(maas *gomaasapi.MAASObject, system_id string, params url.Values) error {
	return maasReleaseNode(maas, system_id, params)
}

// nodeDestroy release a node back into the ready state
func nodeDelete(maas *gomaasapi.MAASObject, system_id string) error {
	return maasDeleteNode(maas, system_id)
}

// nodeUpdate update a node with new information
func nodeUpdate(maas *gomaasapi.MAASObject, system_id string, params url.Values) error {
	log.Println("[DEBUG] [nodeUpdate] Attempting to update a node's data")

	nodeObject, err := maasGetSingleNodeByID(maas, system_id)
	if err != nil {
		log.Printf("[ERROR] [nodeUpdate] Unable to get node (%s) information.\n", system_id)
		return err
	}

	_, err = nodeObject.Update(params)
	if err != nil {
		log.Printf("[ERROR] [nodeUpdate] Unable to update node (%s).  Failed withh error (%s)\n", system_id, err)
		return err
	}
	return nil
}

// parseConstrains parse the provided constraints from terraform into a url.Values that is passed to the API
func parseConstraints(d *schema.ResourceData) (url.Values, error) {
	log.Println("[DEBUG] [parseConstraints] Parsing any existing MAAS constraints")
	retVal := url.Values{}

	hostname, set := d.GetOk("hostname")
	if set {
		log.Printf("[DEBUG] [parseConstraints] setting hostname to %+v", hostname)
		retVal["name"] = strings.Fields(hostname.(string))
	}

	architecture, set := d.GetOk("architecture")
	if set {
		log.Printf("[DEBUG] [parseConstraints] Setting architecture to %s", architecture)
		retVal["arch"] = strings.Fields(architecture.(string))
	}

	cpu_count, set := d.GetOk("cpu_count")
	if set {
		retVal["cpu_count"] = strings.Fields(cpu_count.(string))
	}

	memory, set := d.GetOk("memory")
	if set {
		retVal["memory"] = strings.Fields(memory.(string))
	}

	tags, set := d.GetOk("tags")
	if set {
		tag_strings := make([]string, len(tags.([]interface{})))

		for i := range tags.([]interface{}) {
			tag_strings[i] = tags.([]interface{})[i].(string)
		}
		retVal["tags"] = tag_strings
	}

	//TODO(negronjl): Complete the list based on https://maas.ubuntu.com/docs/api.html

	return retVal, nil
}
