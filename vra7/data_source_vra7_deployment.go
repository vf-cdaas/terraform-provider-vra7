package vra7

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/vmware/terraform-provider-vra7/sdk"
)

func dataSourceVra7Deployment() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceVra7DeploymentRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"deployment_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"catalog_item_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"catalog_item_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"reasons": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"businessgroup_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"businessgroup_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"resource_configuration": resourceConfigurationSchema(false),
			"lease_days": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"lease_start": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"lease_end": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"request_status": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},
			"date_created": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"last_updated": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"owners": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"tenant_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceVra7DeploymentRead(d *schema.ResourceData, meta interface{}) error {
	vraClient := meta.(*sdk.APIClient)

	id, idOk := d.GetOk("id")
	deploymentID, deploymentIDOk := d.GetOk("deployment_id")

	if !idOk && !deploymentIDOk {
		return fmt.Errorf("One of id or deployment_id must be assigned")
	}

	if deploymentID.(string) != "" {
		resource, err := vraClient.GetResource(deploymentID.(string))
		if err != nil {
			return err
		}
		id = resource.RequestID
	}

	requestResourceView, errTemplate := vraClient.GetRequestResourceView(id.(string))
	if errTemplate != nil {
		return errTemplate
	}

	clusterCountMap := make(map[string]int)
	var resourceConfigList []sdk.ResourceConfigurationStruct
	for _, resource := range requestResourceView.Content {
		rMap := resource.(map[string]interface{})
		resourceType := rMap["resourceType"].(string)
		name := rMap["name"].(string)
		dateCreated := ConvertToDateTime(rMap["dateCreated"].(string))
		lastUpdated := ConvertToDateTime(rMap["lastUpdated"].(string))
		resourceID := rMap["resourceId"].(string)
		requestID := rMap["requestId"].(string)
		requestState := rMap["requestState"].(string)

		// if the resource type is VMs, update the resource_configuration attribute
		if resourceType == sdk.InfrastructureVirtual {
			data := rMap["data"].(map[string]interface{})
			componentName := data["Component"].(string)
			parentResourceID := rMap["parentResourceId"].(string)
			var resourceConfigStruct sdk.ResourceConfigurationStruct
			resourceConfigStruct.ResourceState = data
			resourceConfigStruct.ComponentName = componentName
			resourceConfigStruct.Name = name
			resourceConfigStruct.DateCreated = dateCreated
			resourceConfigStruct.LastUpdated = lastUpdated
			resourceConfigStruct.ResourceID = resourceID
			resourceConfigStruct.ResourceType = resourceType
			resourceConfigStruct.RequestID = requestID
			resourceConfigStruct.RequestState = requestState
			resourceConfigStruct.ParentResourceID = parentResourceID
			resourceConfigStruct.IPAddress = data["ip_address"].(string)

			if rMap["description"] != nil {
				resourceConfigStruct.Description = rMap["description"].(string)
			}
			if rMap["status"] != nil {
				resourceConfigStruct.Status = rMap["status"].(string)
			}
			// the cluster value is calculated from the map based on the component name as the
			// resourceViews API does not return that information
			clusterCountMap[componentName] = clusterCountMap[componentName] + 1

			resourceConfigList = append(resourceConfigList, resourceConfigStruct)

		} else if resourceType == sdk.DeploymentResourceType {

			leaseMap := rMap["lease"].(map[string]interface{})
			leaseStart := ConvertToDateTime(leaseMap["start"].(string))
			d.Set("lease_start", leaseStart)
			// if the lease never expires, the end date will be null
			if leaseMap["end"] != nil {
				leaseEnd := ConvertToDateTime(leaseMap["end"].(string))
				d.Set("lease_end", leaseEnd)
			}

			d.Set("catalog_item_id", rMap["catalogItemId"].(string))
			d.Set("catalog_item_name", rMap["catalogItemLabel"].(string))
			d.Set("deployment_id", resourceID)
			d.Set("date_created", dateCreated)
			d.Set("last_updated", lastUpdated)
			d.Set("tenant_id", rMap["tenantId"].(string))
			d.Set("owners", rMap["owners"].([]interface{}))
			d.Set("name", name)
			d.Set("businessgroup_id", rMap["businessGroupId"].(string))
			if rMap["description"] != nil {
				d.Set("description", rMap["description"].(string))
			}
			if rMap["status"] != nil {
				d.Set("request_status", rMap["status"].(string))
			}
		}
	}
	if err := d.Set("resource_configuration", flattenResourceConfigurations(resourceConfigList, clusterCountMap)); err != nil {
		return fmt.Errorf("error setting resource configuration - error: %v", err)
	}
	d.SetId(id.(string))

	log.Info("Finished reading the resource vra7_deployment with request id %s", d.Id())
	return nil
}
