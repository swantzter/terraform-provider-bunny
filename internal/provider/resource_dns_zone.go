package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	bunny "github.com/simplesurance/bunny-go"
)

const (
	keyDnsZoneDomain = "domain"
	keyDnsZoneCustomNameservers = "custom_nameservers"
	keyDnsZoneCustomNameserversEnabled = "enabled"
	keyDnsZoneCustomNameserversSoaEmail = "soa_email"
	keyDnsZoneCustomNameserversNameserver1 = "nameserver_1"
	keyDnsZoneCustomNameserversNameserver2 = "nameserver_2"
	keyDnsZoneLogging = "logging"
	keyDnsZoneLoggingEnabled = "enabled"
	keyDnsZoneLoggingIpAnonymization = "ip_anonymization"
	keyDnsZoneLoggingIpAnonymizationEnabled = "ip_anonymization_enabled"
	keyLastUpdated = "last_updated"
)

func resourceDnsZone() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDnsZoneCreate,
		UpdateContext: resourceDnsZoneUpdate,
		ReadContext:   resourceDnsZoneRead,
		DeleteContext: resourceDnsZoneDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			keyDnsZoneDomain: {
				Type: schema.TypeString
				Description: "The domain for the DNS Zone",
				Required: true,
				ForceNew: true
			},
			keyDnsZoneCustomNameservers: {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				DiffSuppressFunc: diffSupressMissingOptionalBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						keyDnsZoneCustomNameserversEnabled: {
							Type: schema.TypeBool,
							Computed: true,
						},
						keyDnsZoneCustomNameserversSoaEmail: {
							Type: schema.TypeString,
							Required: true
						},
						keyDnsZoneCustomNameserversNameserver1: {
							Type:        schema.TypeString,
							Required:    true,
						},
						keyDnsZoneCustomNameserversNameserver2: {
							Type:        schema.TypeString,
							Required:    true,
						},
					},
				},
			},
			keyDnsZoneLogging: {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				DiffSuppressFunc: diffSupressMissingOptionalBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						keyDnsZoneLoggingEnabled: {
							Type: schema.TypeBool,
							Computed: true,
						},
						keyDnsZoneLoggingIpAnonymizationEnabled: {
							Type: schema.TypeBool,
							Computed: true,
						},
						keyDnsZoneLoggingIpAnonymization: {
							Type: schema.TypeString,
							Optional: true,
							Description: "The type of anonymization.\nValid values: " +
								strings.Join(dnsZoneLoggingAnonymizationTypeKeys, ", "),
							ValidateDiagFunc: validation.ToDiagFunc(
								validation.StringInSlice(dnsZoneLoggingAnonymizationTypeKeys, false),
							),
						},
					},
				},
			},

			keyLastUpdated: {
				Type:     schema.TypeString,
				Computed: true,
			},
		}
	}
}

func resourceDnsZoneCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clt := meta.(*bunny.Client)

	dnsZone, err := clt.DNSZone.Add(ctx, &bunny.DNSZone{
		Domain: d.Get(keyDnsZoneDomain).(string)
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("creating DNS zone failed: %w", err))
	}

	d.SetId(strconv.FormatInt(*dnsZone.ID, 10))
	if err := d.Set(keyLastUpdated, time.Now().Format(time.RFC850)); err != nil {
		return diag.FromErr(err)
	}

	// DNSZone.Add() only supports to set a subset of a DNS Zone object,
	// call Update to set the remaining ones.
	if diags := resourceDnsZoneUpdate(ctx, d, meta); diags.HasError() {
		// if updating fails the dnsZone was still created, initialize with the
		// DNSZone returned from the Add operation
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "setting DNS zone attributes via update failed",
		})

		if err := dnsZoneToResource(dnsZone, d); err != nil {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "converting api-type to resource data failed: " + err.Error(),
			})

		}

		return diags
	}

	return nil
}

func resourceDnsZoneUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clt := meta.(*bunny.Client)

	dnsZone, err := dnsZoneUpdateFromResource(d)
	if err != nil {
		return diagsErrFromErr("converting resource to API type failed", err)
	}

	id, err := getIDAsInt64(d)
	if err != nil {
		return diag.FromErr(err)
	}

	// TODO: Update takes another struct
	updatedDnsZone, err := clt.DNSZone.Update(ctx, id, dnsZone)
	if err != nil {
		return diagsErrFromErr("updating dns zone via API failed", err)
	}

	if err := dnsZoneToResource(updatedDNSZone, d); err != nil {
		return diagsErrFromErr("converting api type to resource data after successful update failed", err)
	}

	if err := d.Set(keyLastUpdated, time.Now().Format(time.RFC850)); err != nil {
		return diagsWarnFromErr(
			fmt.Sprintf("could not set %s", keyLastUpdated),
			err,
		)
	}

	return nil
}

func resourceDnsZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
  clt := meta.(*bunny.Client)

	id, err := getIDAsInt64(d)
	if err != nil {
		return diag.FromErr(err)
	}

	dnsZone, err := clt.DNSZone.Get(ctx, id)
	if err != nil {
		return diagsErrFromErr("could not retrieve DNS zone", err)
	}

	if err := dnsZoneToResource(dnsZone, d); err != nil {
		return diagsErrFromErr("converting api type to resource data after successful read failed", err)
	}

	return nil
}

func resourceDnsZoneDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clt := meta.(*bunny.Client)

	id, err := getIDAsInt64(d)
	if err != nil {
		return diag.FromErr(err)
	}

	err = clt.DNSZone.Delete(ctx, id)
	if err != nil {
		return diagsErrFromErr("could not delete DNS zone", err)
	}

	d.SetId("")

	return nil
}

func dnsZoneToResource(dnsZone *bunny.DNSZone, d *schema.*ResourceData) error {
	if dnsZone.ID != nil {
		d.SetId(strconv.FormatInt(*dnsZone.ID, 10))
	}

	if err := d.Set(keyDnsZoneDomain, dnsZone.Domain); err != nil {
		return err
	}

	customNameserversSettings := map[string]interface{}{}
	customNameserversSettings[keyDnsZoneCustomNameserversEnabled] = dnsZone.CustomNameserversEnabled
	customNameserversSettings[keyDnsZoneCustomNameserversNameserver1] = dnsZone.Nameserver1
	customNameserversSettings[keyDnsZoneCustomNameserversNameserver2] = dnsZone.Nameserver2
	customNameserversSettings[keyDnsZoneCustomNameserversSoaEmail] = dnsZone.SoaEmail
	if err := d.Set(keyDnsZoneCustomNameservers, []map[string]interface{}{customNameserversSettings}); err != nil {
		return err
	}

	loggingSettings := map[string]interface{}{}
	anonymizationType, err := intStrMapGet(dnsZoneLoggingAnonymizationTypesInt, dnsZone.LogAnonymizationType)
		if err != nil {
			return fmt.Errorf("%s: %w", anonymizationType, err)
		}
	loggingSettings[keyDnsZoneLoggingEnabled] = dnsZone.LoggingEnabled
	loggingSettings[keyDnsZoneLoggingIpAnonymizationEnabled] = dnsZone.LoggingIPAnonymizationEnabled
	loggingSettings[keyDnsZoneLoggingIpAnonymization] = anonymizationType
	if err := d.Set(keyDnsZoneLogging, []map[string]interface{}{loggingSettings}); err != nil {
		return err
	}

	return nil
}

func dnsZoneUpdateFromResource(d *schema.ResourceData) (*bunny.DNSZone, error) {
	var res bunny.DNSZoneUpdateOptions

	customNameservers := structureFromResource(d, keyDnsZoneCustomNameservers)
	if len(customNameservers) == 0 {
		res.CustomNameserversEnabled = false
	} else {
		res.CustomNameserversEnabled = true
		res.Nameserver1 = customNameservers.getStrPtr(keyDnsZoneCustomNameserversNameserver1)
		res.Nameserver2 = customNameservers.getStrPtr(keyDnsZoneCustomNameserversNameserver2)
		res.SoaEmail = customNameservers.getStrPtr(keyDnsZoneCustomNameserversSoaEmail)
	}

	logging := structureFromResource(d, keyDnsZoneLogging)
	if len(logging) == 0 {
	  res.LoggingEnabled = false
	  res.LoggingIPAnonymizationEnabled = false
	} else {
		res.LoggingEnabled = true

		anonymizationType, err := edgeRuleTriggerTypeToInt(logging[keyDnsZoneLoggingIpAnonymization].(string))
		if err != nil {
			res.LoggingIPAnonymizationEnabled = false
		} else {
			res.LoggingIPAnonymizationEnabled = true
			res.LogAnonymizationType = anonymizationType
		}

	}

	return &res, nil
}

func dnsZoneLoggingAnonymizationTypeToInt(anonymizationType string) (int, error) {
	if k, exists := dnsZoneLoggingAnonymizationTypesStr[anonymizationType]; exists {
		return k, nil
	}

	return -1, fmt.Errorf("unsupported IP anonymization type: %q", triggerType)
}
