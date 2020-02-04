package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/juju/gomaasapi"
)

func makeCreateMachineArgs(d *schema.ResourceData) gomaasapi.CreateMachineArgs {
	args := gomaasapi.CreateMachineArgs{
		Commission:   false, // we manage the commision state
		MACAddresses: []string{},
	}
	args.UpdateMachineArgs = makeUpdateMachineArgs(d)

	if architecture, ok := d.GetOk("architecture"); ok {
		args.Architecture = architecture.(string)
	}

	if description, ok := d.GetOk("description"); ok {
		args.Description = description.(string)
	}

	if macAddress, ok := d.GetOk("mac_address"); ok {
		args.MACAddresses = []string{macAddress.(string)}
	}

	return args
}

func makeUpdateMachineArgs(d *schema.ResourceData) gomaasapi.UpdateMachineArgs {
	args := gomaasapi.UpdateMachineArgs{
		PowerOpts: map[string]string{},
	}

	if hostname, ok := d.GetOk("hostname"); ok {
		args.Hostname = hostname.(string)
	}

	if domain, ok := d.GetOk("domain"); ok {
		args.Domain = domain.(string)
	}

	const powerPrefix = "power.0"
	if _, ok := d.GetOk(powerPrefix); ok {
		if ptype, ok := d.GetOk(powerPrefix + ".type"); ok {
			args.PowerType = ptype.(string)
		}

		if user, ok := d.GetOk(powerPrefix + ".user"); ok {
			args.PowerUser = user.(string)
		}

		if password, ok := d.GetOk(powerPrefix + ".password"); ok {
			args.PowerPassword = password.(string)
		}

		if address, ok := d.GetOk(powerPrefix + ".address"); ok {
			args.PowerAddress = address.(string)
		}

		if custom, ok := d.GetOk(powerPrefix + ".custom"); ok {
			values := custom.(map[string]interface{})
			for k, v := range values {
				args.PowerOpts[k] = v.(string)
			}
		}
	}
	return args
}

func updateMachineInterfaces(d *schema.ResourceData, controller gomaasapi.Controller, machine gomaasapi.Machine) error {
	// enumerate the subnets available
	cidrToSubnet := map[string]gomaasapi.Subnet{}
	spaces, err := controller.Spaces()
	if err != nil {
		return err
	}
	for _, space := range spaces {
		// Note: this will collapse subnets that overlap in different spaces
		// TODO: link up spaces better
		for _, subnet := range space.Subnets() {
			log.Printf("[DEBUG] Found CIDR %s in Space %s", space.Name(), subnet.CIDR())
			cidrToSubnet[subnet.CIDR()] = subnet
		}
	}

	// Build a mapping of interface name to ID
	nameToIface := map[string]gomaasapi.Interface{}
	for _, iface := range machine.InterfaceSet() {
		nameToIface[iface.Name()] = iface
	}

	for i := 0; i < d.Get("interface.#").(int); i++ {
		name := d.Get(fmt.Sprintf("interface.%d.name", i)).(string)
		log.Printf("[DEBUG] [resourceMAASMachineCreate] Updating interface %s", name)
		if bondBlock, ok := d.GetOk(fmt.Sprintf("interface.%d.name.bond.0", i)); ok {
			bondParams := bondBlock.(*schema.ResourceData)
			log.Printf("[DEBUG] [resourceMAASMachineCreate] Creating bond %s", name)
			// create a new bond device
			args := gomaasapi.CreateMachineBondArgs{
				UpdateInterfaceArgs: gomaasapi.UpdateInterfaceArgs{
					BondMode:           bondParams.Get("mode").(string),
					MACAddress:         bondParams.Get("mac_address").(string),
					BondMiimon:         bondParams.Get("miimon").(int),
					BondDownDelay:      bondParams.Get("downdelay").(int),
					BondUpDelay:        bondParams.Get("updelay").(int),
					BondLACPRate:       bondParams.Get("lacp_rate").(string),
					BondXmitHashPolicy: bondParams.Get("xmit_hash_policy").(string),
				},
				Parents: []gomaasapi.Interface{},
			}

			if parents, ok := bondParams.GetOk("parents"); ok {
				for _, parent := range parents.([]interface{}) {
					args.Parents = append(args.Parents, parent.(gomaasapi.Interface))
				}
			}

			bondIface, err := machine.CreateBond(args)
			if err != nil {
				return fmt.Errorf("Failed to create bond: %v", err)
			}
			nameToIface[name] = bondIface
		}

		if iface, ok := nameToIface[name]; ok {
			// link the device to a subnet
			subnetCIDR := d.Get(fmt.Sprintf("interface.%d.subnet", i)).(string)
			subnet, ok := cidrToSubnet[subnetCIDR]
			if !ok {
				return fmt.Errorf("No subnet CIDR %s exists", subnetCIDR)
			}

			// unlink first
			err := iface.UnlinkSubnet(subnet)
			if err != nil {
				return err
			}

			// now link the correct subnet
			mode := d.Get(fmt.Sprintf("interface.%d.mode", i)).(string)
			log.Printf("[DEBUG] [resourceMAASMachineCreate] Linking interface %s to subnet %s (mode: %s)", name, subnetCIDR, mode)
			args := gomaasapi.LinkSubnetArgs{
				Mode:   gomaasapi.InterfaceLinkMode(mode),
				Subnet: subnet,
			}

			err = iface.LinkSubnet(args)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// resourceMAASMachineCreate Manages the commisioning of a new maas node
func resourceMAASMachineCreate(d *schema.ResourceData, meta interface{}) error {
	log.Println("[DEBUG] [resourceMAASMachineCreate] Launching new machine")

	controller := meta.(*Config).Controller

	// Attempt to create a new device (it might already exist)

	createArgs := makeCreateMachineArgs(d)
	_, err := controller.CreateMachine(createArgs)
	if err != nil {
		// is error "already exists?"
		log.Printf("[ERROR] [resourceMAASMachineCreate] Creating a device failed, it might already exist: %v.", err)
	}

	macAddressVal, set := d.GetOk("mac_address")
	if !set {
		return fmt.Errorf("Missing mac_address value")
	}
	macAddress, ok := macAddressVal.(string)
	if !ok {
		return fmt.Errorf("Invalid type for mac_address field")
	}

	if macAddress == "" {
		return fmt.Errorf("Empty mac_address value")
	}

	// Locate the machine we either just created or was already auto-created
	machines, err := controller.Machines(gomaasapi.MachinesArgs{MACAddresses: []string{macAddress}})
	if err != nil {
		log.Printf("[ERROR] [resourceMAASMachineCreate] Unable to seach machines by mac: %v.", err)
		return err
	}
	if len(machines) == 0 {
		log.Printf("[DEBUG] [resourceMAASMachineCreate] no machine with mac: %v.", macAddress)
		return fmt.Errorf("Failed to create or locate machine with mac %s", macAddress)
	}
	machine := machines[0]

	d.SetId(machine.SystemID())

	// update base machine options
	machineArgs := makeUpdateMachineArgs(d)
	err = machine.Update(machineArgs)
	if err != nil {
		log.Println("[DEBUG] Unable to update machine")
		return fmt.Errorf("Failed to update machine options: %v", err)
	}

	// add tags
	if tags, ok := d.GetOk("tags"); ok {
		for _, item := range tags.(*schema.Set).List() {
			err := machineUpdateTags(meta.(*Config).Controller, machine, item.(string))
			if err != nil {
				log.Printf("[ERROR] Unable to update machine (%s) with tag (%s)", d.Id(), item.(string))
			}
		}
	}

	commissionArgs := gomaasapi.CommissionArgs{
		EnableSSH:            d.Get("enable_ssh").(bool),
		SkipBMCConfig:        d.Get("skip_bmc_config").(bool),
		SkipNetworking:       d.Get("skip_networking").(bool),
		SkipStorage:          d.Get("skip_storage").(bool),
		CommissioningScripts: []string{},
		TestingScripts:       []string{},
	}

	if scripts, ok := d.GetOk("commissioning_scripts"); ok {
		commissionArgs.CommissioningScripts = scripts.([]string)
	}
	if scripts, ok := d.GetOk("testing_scripts"); ok {
		commissionArgs.TestingScripts = scripts.([]string)
	}

	if err := machine.Commission(commissionArgs); err != nil {
		log.Printf("[ERROR] [resourceMAASMachineCreate] Unable to commission: %s\n", d.Id())
		_, stateName, _ := getMachineStatus(controller, machine.SystemID())()
		if stateName != "Commissioning" {
			// we were in a real unexpected state - bail
			log.Printf("[ERROR] [resourceMAASMachineCreate] commision request machine state: '%s'\n", stateName)
			return err
		}
		// ignore this error, we may have auto-entered commissioning state, not great but ok :|
	}

	log.Printf("[DEBUG] [resourceMAASMachineCreate] Waiting for commisioning (%s) to complete\n", d.Id())
	waitToCommissionConf := &resource.StateChangeConf{
		Pending:    []string{"Commissioning", "Testing"},
		Target:     []string{"Ready"},
		Refresh:    getMachineStatus(meta.(*Config).Controller, machine.SystemID()),
		Timeout:    25 * time.Minute,
		Delay:      60 * time.Second,
		MinTimeout: 30 * time.Second,
	}

	commissionedMachine, err := waitToCommissionConf.WaitForState()
	if err != nil {
		return fmt.Errorf("Failed waiting for commisioning (%s) to complete: %s", d.Id(), err)
	}

	err = updateMachineInterfaces(d, controller, commissionedMachine.(gomaasapi.Machine))
	if err != nil {
		return err
	}

	// release the machine so it can be deployed by another user
	err = controller.ReleaseMachines(gomaasapi.ReleaseMachinesArgs{SystemIDs: []string{machine.SystemID()}})
	if err != nil {
		return err
	}
	log.Printf("[DEBUG] [resourceMAASMachineCreate] Waiting for release (%s) to complete\n", d.Id())
	releaseConf := &resource.StateChangeConf{
		Pending:    []string{"Releasing"},
		Target:     []string{"Ready"},
		Refresh:    getMachineStatus(meta.(*Config).Controller, machine.SystemID()),
		Timeout:    5 * time.Minute,
		Delay:      60 * time.Second,
		MinTimeout: 30 * time.Second,
	}

	_, err = releaseConf.WaitForState()
	if err != nil {
		return fmt.Errorf("Failed waiting for release (%s) to complete: %s", d.Id(), err)
	}

	return resourceMAASMachineRead(d, meta)
}

// resourceMAASMachineRead read node information from a maas node
func resourceMAASMachineRead(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] Reading machine (%s) information.\n", d.Id())

	controller := meta.(*Config).Controller
	machine, err := controller.GetMachine(d.Id())
	if err != nil {
		return err
	}

	d.Set("architecture", machine.Architecture())
	d.Set("hostname", machine.Hostname())
	d.Set("domain", strings.SplitN(machine.FQDN(), ".", 2)[1])
	d.Set("mac_address", machine.BootInterface().MACAddress())

	iface := machine.BootInterface()
	if iface != nil {
		ifaceLinks := iface.Links()
		if len(ifaceLinks) > 0 {
			d.Set("interface", []map[string]interface{}{
				{
					"name":   iface.Name(),
					"mode":   strings.ToUpper(ifaceLinks[0].Mode()),
					"subnet": ifaceLinks[0].Subnet().CIDR(),
				},
			})
		}
	}

	d.Set("tags", machine.Tags())

	// TODO: how do we handle auto-configured power management
	// like iDRAC or IPMI?
	//
	//d.Set("power", machine.Power())

	log.Printf("[DEBUG] Done reading machine %s", d.Id())

	return nil
}

// resourceMAASMachineUpdate update a node in terraform state
func resourceMAASMachineUpdate(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] [resourceMAASMachineUpdate] Modifying machine %s\n", d.Id())

	controller := meta.(*Config).Controller
	machine, err := controller.GetMachine(d.Id())
	if err != nil {
		return err
	}

	d.Partial(true)
	updateArgs := gomaasapi.UpdateMachineArgs{}
	needsUpdate := false
	if d.HasChange("hostname") {
		updateArgs.Hostname = d.Get("hostname").(string)
		needsUpdate = true
	}
	if d.HasChange("domain") {
		updateArgs.Domain = d.Get("domain").(string)
		needsUpdate = true
	}
	if needsUpdate {
		err := machine.Update(updateArgs)
		if err != nil {
			return err
		}
	}

	if d.HasChange("tags") {
		hasTags := map[string]gomaasapi.Tag{}
		for _, t := range machine.Tags() {
			tag, err := controller.GetTag(t)
			if err != nil {
				return err
			}
			hasTags[t] = tag
		}
		wantTags := map[string]struct{}{}
		for _, t := range d.Get("tags").([]interface{}) {
			wantTags[t.(string)] = struct{}{}
		}
		// add any missing tags
		for wantTag := range wantTags {
			_, has := hasTags[wantTag]
			if !has {
				var maasTag gomaasapi.Tag
				maasTag, err = controller.GetTag(wantTag)
				if err != nil {
					log.Printf("[DEBUG] Creating new MaaS tag %s", wantTag)
					maasTag, err = controller.CreateTag(gomaasapi.CreateTagArgs{Name: wantTag})
					if err != nil {
						return fmt.Errorf("Failed to get or create tag %s: %v", wantTag, err)
					}
				}
				log.Printf("[DEBUG] Adding tag %s to %s", maasTag.Name(), machine.Hostname())
				err := maasTag.AddToMachine(machine.SystemID())
				if err != nil {
					return fmt.Errorf("Failed to add tag %s to %s", wantTag, machine.Hostname())
				}
			}
		}
		// remove any extra tags
		for name, hasTag := range hasTags {
			_, doesWant := wantTags[name]
			if !doesWant {
				log.Printf("[DEBUG] Removing extra tag %s from %s", name, machine.Hostname())
				hasTag.RemoveFromMachine(machine.SystemID())
			}

		}
	}

	// TODO: power
	d.Partial(false)

	log.Printf("[DEBUG] Done Modifying machine %s", d.Id())
	return nil
}

// resourceMAASDeploymentDelete will release the commisioning
func resourceMAASMachineDelete(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] Deleting node %s\n", d.Id())
	controller := meta.(*Config).Controller
	machines, err := controller.Machines(gomaasapi.MachinesArgs{SystemIDs: []string{d.Id()}})
	if err != nil {
		log.Printf("[ERROR] Unable to delete machine (%s): %v", d.Id(), err)
	}
	if len(machines) == 0 {
		return fmt.Errorf("Machine with id %s does not exist", d.Id())
	}
	err = machines[0].Delete()
	log.Printf("[DEBUG] [resourceMAASMachineDelete] machine (%s) deleted", d.Id())
	d.SetId("")
	return nil
}
