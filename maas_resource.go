package main

import (
	"crypto/sha1"
	"encoding/hex"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
)

// resourceMAASDeployment creates a new terraform schema resource
func resourceMAASDeployment() *schema.Resource {
	log.Println("[DEBUG] [resourceMAASDeployment] Initializing data structure")
	return &schema.Resource{
		Create: resourceMAASDeploymentCreate,
		Read:   resourceMAASDeploymentRead,
		Update: resourceMAASDeploymentUpdate,
		Delete: resourceMAASDeploymentDelete,

		SchemaVersion: 1,

		Schema: map[string]*schema.Schema{
			"architecture": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"boot_type": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"cpu_count": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"disable_ipv4": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"distro_series": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"hostname": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"deploy_hostname": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"deploy_tags": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"release_erase": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: false,
				Default:  true,
			},

			"release_erase_secure": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: false,
				Default:  false,
			},

			"release_erase_quick": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: false,
				Default:  false,
			},

			"ip_addresses": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"macaddress_set": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"mac_address": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"resource_uri": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},

			"memory": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"netboot": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"osystem": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"owner": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"physicalblockdevice_set": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"block_size": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"id": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"id_path": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"model": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"path": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"serial": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"size": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"tags": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},

			"power_state": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"power_type": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"pxe_mac": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"mac_address": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"resource_uri": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},

			"resource_uri": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"routers": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"status": {
				Type:     schema.TypeInt,
				Optional: true,
			},

			"storage": {
				Type:     schema.TypeInt,
				Optional: true,
			},

			"swap_size": {
				Type:     schema.TypeInt,
				Optional: true,
			},

			"system_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"tag_names": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"zone": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"description": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"resource_uri": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},

			"user_data": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				StateFunc: func(v interface{}) string {
					switch v.(type) {
					case string:
						hash := sha1.Sum([]byte(v.(string)))
						return hex.EncodeToString(hash[:])
					default:
						return ""
					}
				},
			},

			"hwe_kernel": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"comment": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

// resourceMAASNode creates a new terraform schema resource
func resourceMAASNode() *schema.Resource {
	log.Println("[DEBUG] [resourceMAASNode] Initializing data structure")
	return &schema.Resource{
		Create: resourceMAASNodeCreate,
		Read:   resourceMAASNodeRead,
		Update: resourceMAASNodeUpdate,
		Delete: resourceMAASNodeDelete,

		SchemaVersion: 1,

		Schema: map[string]*schema.Schema{
			"mac_address": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"domain": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: false,
			},
			"hostname": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: false,
			},
			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			/*
			   * interface (op=link_subnet)
			     * subnet
			     * mode
			   * power management
			*/
		},
	}
}

// TODO: add maas_node schema
