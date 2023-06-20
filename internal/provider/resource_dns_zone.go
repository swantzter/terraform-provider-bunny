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
	keyLastUpdated = "last_updated"
)

func resourceDnsZone() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDnsZoneCreate,
		UpdateContext: resourceDnsZoneUpdate,
		ReadContext:   resourceDnsZoneRead,
		DeleteContext: resourceDnsZoneDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceDnsZoneImport,
		},

		Schema: map[string]*schema.Schema{
			keyDnsZoneDomain: {
				Type: schema.TypeString
				Description: "The domain for the DNS Zone",
				Required: true,
				ForceNew: true
			},

			// TODO: custom nameserver, specify as blocks? (ns1, ns2, strings)
			// TODO: SoaEmail (string)
			// TODO: LoggingEnabled (bool)
			// TODO: LogAnonymizationType (0, 1)
			// TODO: LoggingIPAnonymizationEnabled (bool)

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

	dnsZone, err := dnsZoneFromResource(d)
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

	if err := dnsZoneToResource(pz, d); err != nil {
		return diagsErrFromErr("converting api type to resource data after successful read failed", err)
	}

	return nil
}
