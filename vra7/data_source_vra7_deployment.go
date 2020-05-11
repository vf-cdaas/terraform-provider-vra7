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
			"resource_configuration": dataResourceConfigurationSchema(),
			"lease_days": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"request_status": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},
			"created_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"expiry_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"owners": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
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

	//
	if id.(string) != "" {
		depID, err := vraClient.GetDeploymentIDFromRequest(id.(string))
		if err != nil {
			return err
		}
		deploymentID = depID
	}

	requestID := ""
	if deploymentID.(string) != "" {
		resource, err := vraClient.GetResource(deploymentID.(string))
		if err != nil {
			return err
		}
		requestID = resource.RequestID
	}

	// Since the resource view API above do not provide the cluster value, it is calculated
	// by tracking the component name and updated in the state file
	clusterCountMap := make(map[string]int)
	// parse the resource view API response and create a resource configuration list that will contain information
	// of the deployed VMs
	var resourceConfigList []sdk.ResourceConfigurationStruct
	for _, resource := range requestResourceView.Content {
		rMap := resource.(map[string]interface{})
		resourceType := rMap["resourceType"].(string)
		name := rMap["name"].(string)
		dateCreated := rMap["dateCreated"].(string)
		lastUpdated := rMap["lastUpdated"].(string)
		resourceID := rMap["resourceId"].(string)
		requestID := rMap["requestId"].(string)
		requestState := rMap["requestState"].(string)
		hasChildren := rMap["hasChildren"].(bool)

		// only read the Deployment from the first content response,
		// get VMs and other components as Child Resources from the Deployment
		if resourceType == sdk.DeploymentResourceType {

			leaseMap := rMap["lease"].(map[string]interface{})
			leaseStart := leaseMap["start"].(string)
			d.Set("lease_start", leaseStart)
			// if the lease never expires, the end date will be null
			if leaseMap["end"] != nil {
				leaseEnd := leaseMap["end"].(string)
				d.Set("lease_end", leaseEnd)
				// the lease_days are calculated from the current time and lease_end dates as the resourceViews API does not return that information
				currTime, _ := time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
				endTime, _ := time.Parse(time.RFC3339, leaseEnd)
				diff := endTime.Sub(currTime)
				d.Set("lease_days", int(diff.Hours()/24))
				// end
			} else {
				d.Set("lease_days", nil) // set lease days to nil if lease_end is nil
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

			if hasChildren {
				links := rMap["links"].([]interface{})
				var childResourcesLink string

				for _, link := range links {
					l := link.(map[string]interface{})
					if l["rel"].(string) == "GET: Child Resources" {
						childResourcesLink = l["href"].(string)
					}
				}
				requestResourceView, errTemplate := vraClient.GetChildResources(childResourcesLink)
				if requestResourceView != nil && len(requestResourceView.Content) == 0 {
					//If resource does not exists then unset the resource ID from state file
					d.SetId("")
					return fmt.Errorf("The resource cannot be found")
				}
				if errTemplate != nil || len(requestResourceView.Content) == 0 {
					return fmt.Errorf("Resource view failed to load with the error %v", errTemplate)
				}

				for _, resource := range requestResourceView.Content {
					rMap := resource.(map[string]interface{})
					resourceType := rMap["resourceType"].(string)
					name := rMap["name"].(string)
					dateCreated := rMap["dateCreated"].(string)
					lastUpdated := rMap["lastUpdated"].(string)
					resourceID := rMap["resourceId"].(string)
					// if the resource type is VMs, update the resource_configuration attribute
					data := rMap["data"].(map[string]interface{})
					// componentName := data["Component"].(string)
					parentResourceID := rMap["parentResourceId"].(string)
					var resourceConfigStruct sdk.ResourceConfigurationStruct
					resourceConfigStruct.Configuration = data
					// resourceConfigStruct.ComponentName = componentName
					resourceConfigStruct.Name = name
					resourceConfigStruct.DateCreated = dateCreated
					resourceConfigStruct.LastUpdated = lastUpdated
					resourceConfigStruct.ResourceID = resourceID
					resourceConfigStruct.ResourceType = resourceType
					resourceConfigStruct.RequestID = requestID
					resourceConfigStruct.RequestState = requestState
					resourceConfigStruct.ParentResourceID = parentResourceID
					if resourceType == sdk.InfrastructureVirtual {
						resourceConfigStruct.IPAddress = data["ip_address"].(string)
					}

					if rMap["description"] != nil {
						resourceConfigStruct.Description = rMap["description"].(string)
					}
					if rMap["status"] != nil {
						resourceConfigStruct.Status = rMap["status"].(string)
					}
					// the cluster value is calculated from the map based on the component name as the
					// resourceViews API does not return that information
					// clusterCountMap[componentName] = clusterCountMap[componentName] + 1

					resourceConfigList = append(resourceConfigList, resourceConfigStruct)
				}

			}

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

	d.SetId(requestID)

	log.Info("Finished reading the data source vra7_deployment with request id %s", d.Id())
	return nil
}
