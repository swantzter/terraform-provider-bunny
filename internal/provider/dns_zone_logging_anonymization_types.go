package provider

var dnsZoneLoggingAnonymizationTypesStr = map[string]int{
	"remove_octet": 0,
	"drop_ip":      1,
}

var dnsZoneLoggingAnonymizationTypesInt = reverseStrIntMap(dnsZoneLoggingAnonymizationTypesStr)

var dnsZoneLoggingAnonymizationTypeKeys = strIntMapKeysSorted(dnsZoneLoggingAnonymizationTypesStr)
