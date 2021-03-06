package google

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"google.golang.org/api/compute/v1"
)

func resourceComputeProjectMetadata() *schema.Resource {
	return &schema.Resource{
		Create: resourceComputeProjectMetadataCreate,
		Read:   resourceComputeProjectMetadataRead,
		Update: resourceComputeProjectMetadataUpdate,
		Delete: resourceComputeProjectMetadataDelete,

		SchemaVersion: 0,

		Schema: map[string]*schema.Schema{
			"metadata": &schema.Schema{
				Elem:     schema.TypeString,
				Type:     schema.TypeMap,
				Required: true,
			},

			"project": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceComputeProjectMetadataCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	projectID, err := getProject(d, config)
	if err != nil {
		return err
	}

	createMD := func() error {
		// Load project service
		log.Printf("[DEBUG] Loading project service: %s", projectID)
		project, err := config.clientCompute.Projects.Get(projectID).Do()
		if err != nil {
			return fmt.Errorf("Error loading project '%s': %s", projectID, err)
		}

		md := project.CommonInstanceMetadata

		newMDMap := d.Get("metadata").(map[string]interface{})
		// Ensure that we aren't overwriting entries that already exist
		for _, kv := range md.Items {
			if _, ok := newMDMap[kv.Key]; ok {
				return fmt.Errorf("Error, key '%s' already exists in project '%s'", kv.Key, projectID)
			}
		}

		// Append new metadata to existing metadata
		for key, val := range newMDMap {
			v := val.(string)
			md.Items = append(md.Items, &compute.MetadataItems{
				Key:   key,
				Value: &v,
			})
		}

		op, err := config.clientCompute.Projects.SetCommonInstanceMetadata(projectID, md).Do()

		if err != nil {
			return fmt.Errorf("SetCommonInstanceMetadata failed: %s", err)
		}

		log.Printf("[DEBUG] SetCommonMetadata: %d (%s)", op.Id, op.SelfLink)

		return computeOperationWait(config, op, project.Name, "SetCommonMetadata")
	}

	err = MetadataRetryWrapper(createMD)
	if err != nil {
		return err
	}

	return resourceComputeProjectMetadataRead(d, meta)
}

func resourceComputeProjectMetadataRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	projectID, err := getProject(d, config)
	if err != nil {
		return err
	}

	// Load project service
	log.Printf("[DEBUG] Loading project service: %s", projectID)
	project, err := config.clientCompute.Projects.Get(projectID).Do()
	if err != nil {
		return handleNotFoundError(err, d, fmt.Sprintf("Project metadata for project %q", projectID))
	}

	md := project.CommonInstanceMetadata

	if err = d.Set("metadata", MetadataFormatSchema(d.Get("metadata").(map[string]interface{}), md)); err != nil {
		return fmt.Errorf("Error setting metadata: %s", err)
	}

	d.SetId("common_metadata")

	return nil
}

func resourceComputeProjectMetadataUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	projectID, err := getProject(d, config)
	if err != nil {
		return err
	}

	if d.HasChange("metadata") {
		o, n := d.GetChange("metadata")

		updateMD := func() error {
			// Load project service
			log.Printf("[DEBUG] Loading project service: %s", projectID)
			project, err := config.clientCompute.Projects.Get(projectID).Do()
			if err != nil {
				return fmt.Errorf("Error loading project '%s': %s", projectID, err)
			}

			md := project.CommonInstanceMetadata

			MetadataUpdate(o.(map[string]interface{}), n.(map[string]interface{}), md)

			op, err := config.clientCompute.Projects.SetCommonInstanceMetadata(projectID, md).Do()

			if err != nil {
				return fmt.Errorf("SetCommonInstanceMetadata failed: %s", err)
			}

			log.Printf("[DEBUG] SetCommonMetadata: %d (%s)", op.Id, op.SelfLink)

			// Optimistic locking requires the fingerprint received to match
			// the fingerprint we send the server, if there is a mismatch then we
			// are working on old data, and must retry
			return computeOperationWait(config, op, project.Name, "SetCommonMetadata")
		}

		err := MetadataRetryWrapper(updateMD)
		if err != nil {
			return err
		}

		return resourceComputeProjectMetadataRead(d, meta)
	}

	return nil
}

func resourceComputeProjectMetadataDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	projectID, err := getProject(d, config)
	if err != nil {
		return err
	}

	// Load project service
	log.Printf("[DEBUG] Loading project service: %s", projectID)
	project, err := config.clientCompute.Projects.Get(projectID).Do()
	if err != nil {
		return fmt.Errorf("Error loading project '%s': %s", projectID, err)
	}

	md := project.CommonInstanceMetadata

	// Remove all items
	md.Items = nil

	op, err := config.clientCompute.Projects.SetCommonInstanceMetadata(projectID, md).Do()

	if err != nil {
		return fmt.Errorf("Error removing metadata from project %s: %s", projectID, err)
	}

	log.Printf("[DEBUG] SetCommonMetadata: %d (%s)", op.Id, op.SelfLink)

	err = computeOperationWait(config, op, project.Name, "SetCommonMetadata")
	if err != nil {
		return err
	}

	return resourceComputeProjectMetadataRead(d, meta)
}
