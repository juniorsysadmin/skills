// Command aws_pricing queries the AWS Price List Query API for the aws-pricing
// skill (see ../SKILL.md).
//
// Five subcommands: service-codes, service-attributes, attribute-values,
// pricing, price-list-urls. Unlike the stdlib-only aws-docs helper this one
// depends on aws-sdk-go-v2, because the Price List API needs SigV4 with the
// full credential chain (profiles, SSO).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

// The Price List API exists only in these regions; pricingRegion maps anything
// else to the nearest one by prefix.
var pricingRegions = []string{
	"us-east-1",
	"eu-central-1",
	"ap-southeast-1",
	"ap-south-1",
	"cn-northwest-1",
}

// AWS rejects an ANY_OF/NONE_OF filter whose value list exceeds this length.
const anyOfValueLimit = 1024

// stringSlice is a repeatable string flag (e.g. --filter a=b --filter c=d).
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// fail prints the error (with a reauth hint when it smells like credentials)
// and exits non-zero; hint may be empty.
func fail(msg, hint string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	lower := strings.ToLower(msg)
	for _, needle := range []string{"credential", "expiredtoken", "token has expired"} {
		if strings.Contains(lower, needle) {
			fmt.Fprintln(os.Stderr, "hint: no valid AWS credentials — check AWS_PROFILE or run `aws sso login`")
			break
		}
	}
	if hint != "" {
		fmt.Fprintln(os.Stderr, "hint: "+hint)
	}
	os.Exit(1)
}

func compileRegex(pattern string) *regexp.Regexp {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		fail(fmt.Sprintf("invalid regex %q: %v", pattern, err), "")
	}
	return re
}

func splitKV(arg, flagName string) (string, string) {
	field, value, found := strings.Cut(arg, "=")
	if !found {
		fail(fmt.Sprintf("%s needs FIELD=VALUE, got %q", flagName, arg), "")
	}
	return field, value
}

// matches treats a nil regex as match-everything.
func matches(re *regexp.Regexp, s string) bool {
	return re == nil || re.MatchString(s)
}

// pricingRegion maps any region to the nearest region hosting the Price List API.
func pricingRegion(requested string) string {
	switch {
	case requested == "":
		return "us-east-1"
	case slices.Contains(pricingRegions, requested):
		return requested
	case strings.HasPrefix(requested, "cn-"):
		return "cn-northwest-1"
	case strings.HasPrefix(requested, "eu-"),
		strings.HasPrefix(requested, "me-"),
		strings.HasPrefix(requested, "af-"):
		return "eu-central-1"
	case strings.HasPrefix(requested, "ap-"):
		return "ap-south-1"
	}
	return "us-east-1"
}

// makeClient pins the client to a pricing region; the ambient AWS region never applies.
func makeClient(ctx context.Context, requestedRegion string) *pricing.Client {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(pricingRegion(requestedRegion)))
	if err != nil {
		fail(fmt.Sprintf("loading AWS config failed: %v", err), "")
	}
	return pricing.NewFromConfig(cfg)
}

// parseArgs runs fs.Parse iteratively so positionals may appear before,
// between, or after flags; Go's flag package alone stops at the first
// non-flag argument.
func parseArgs(fs *flag.FlagSet, args []string) []string {
	var positionals []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			os.Exit(2)
		}
		if fs.NArg() == 0 {
			break
		}
		positionals = append(positionals, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	return positionals
}

func requireServiceCode(positionals []string, cmd string) string {
	if len(positionals) != 1 {
		fail(fmt.Sprintf("%s takes exactly one SERVICE_CODE argument", cmd), "")
	}
	return positionals[0]
}

func runServiceCodes(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("service-codes", flag.ExitOnError)
	filter := fs.String("filter", "", "Case-insensitive regex to filter codes")
	if extra := parseArgs(fs, args); len(extra) != 0 {
		fail("service-codes takes no positional arguments", "")
	}
	var re *regexp.Regexp
	if *filter != "" {
		re = compileRegex(*filter)
	}

	client := makeClient(ctx, "")
	var codes []string
	pager := pricing.NewDescribeServicesPaginator(client, &pricing.DescribeServicesInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			fail(fmt.Sprintf("DescribeServices failed: %v", err), "")
		}
		for _, service := range page.Services {
			if code := aws.ToString(service.ServiceCode); matches(re, code) {
				codes = append(codes, code)
			}
		}
	}
	sort.Strings(codes)
	codes = slices.Compact(codes)
	if len(codes) == 0 {
		fail("no service codes matched",
			"loosen or drop --filter; run without --filter to see every code")
	}
	fmt.Println(strings.Join(codes, "\n"))
}

func runServiceAttributes(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("service-attributes", flag.ExitOnError)
	filter := fs.String("filter", "", "Case-insensitive regex to filter names")
	serviceCode := requireServiceCode(parseArgs(fs, args), "service-attributes")
	var re *regexp.Regexp
	if *filter != "" {
		re = compileRegex(*filter)
	}

	client := makeClient(ctx, "")
	resp, err := client.DescribeServices(ctx, &pricing.DescribeServicesInput{
		ServiceCode: aws.String(serviceCode),
	})
	if err != nil {
		fail(fmt.Sprintf("DescribeServices(%s) failed: %v", serviceCode, err),
			"verify the service code with `service-codes`")
	}
	if len(resp.Services) == 0 {
		fail(fmt.Sprintf("unknown service code %q", serviceCode),
			"verify the service code with `service-codes`")
	}
	var names []string
	for _, name := range resp.Services[0].AttributeNames {
		if matches(re, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		fail(fmt.Sprintf("no attributes matched for %s", serviceCode), "loosen or drop --filter")
	}
	fmt.Println(strings.Join(names, "\n"))
}

// attributeValues pages through one attribute's values, regex-filtered, sorted
// and deduplicated.
func attributeValues(ctx context.Context, client *pricing.Client, serviceCode, attr string, re *regexp.Regexp) []string {
	var values []string
	pager := pricing.NewGetAttributeValuesPaginator(client, &pricing.GetAttributeValuesInput{
		ServiceCode:   aws.String(serviceCode),
		AttributeName: aws.String(attr),
		MaxResults:    aws.Int32(100),
	})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			fail(fmt.Sprintf("GetAttributeValues(%s, %s) failed: %v", serviceCode, attr, err),
				"verify the names with `service-codes` and `service-attributes`")
		}
		for _, v := range page.AttributeValues {
			if value := aws.ToString(v.Value); value != "" && matches(re, value) {
				values = append(values, value)
			}
		}
	}
	sort.Strings(values)
	return slices.Compact(values)
}

func runAttributeValues(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("attribute-values", flag.ExitOnError)
	var filters stringSlice
	fs.Var(&filters, "filter", "Case-insensitive regex per attribute as ATTR=REGEX (repeatable)")
	positionals := parseArgs(fs, args)
	if len(positionals) < 2 {
		fail("attribute-values takes SERVICE_CODE and at least one ATTRIBUTE", "")
	}
	serviceCode, attributes := positionals[0], positionals[1:]
	regexByAttr := map[string]*regexp.Regexp{}
	for _, f := range filters {
		attr, pattern := splitKV(f, "--filter")
		regexByAttr[attr] = compileRegex(pattern)
	}

	// Buffer everything so a failure on any attribute produces no output at all.
	client := makeClient(ctx, "")
	var out strings.Builder
	for _, attr := range attributes {
		out.WriteString("# " + attr + "\n")
		for _, v := range attributeValues(ctx, client, serviceCode, attr, regexByAttr[attr]) {
			out.WriteString(v + "\n")
		}
	}
	fmt.Print(out.String())
}

func buildFilter(field string, filterType types.FilterType, value string) types.Filter {
	if (filterType == types.FilterTypeAnyOf || filterType == types.FilterTypeNoneOf) &&
		len(value) > anyOfValueLimit {
		fail(fmt.Sprintf("%s value list is %d characters; AWS caps ANY_OF/NONE_OF at %d",
			field, len(value), anyOfValueLimit),
			"split the query into several calls with shorter value lists")
	}
	return types.Filter{
		Field: aws.String(field),
		Type:  filterType,
		Value: aws.String(value),
	}
}

// pricingFlags carries every pricing flag so the command splits into helpers.
type pricingFlags struct {
	regions, filterEquals, filterAny, filterContains, filterNone stringSlice
	terms, attrs                                                 string
	excludeFree                                                  bool
	maxResults                                                   int
	nextToken                                                    string
	maxChars                                                     int
}

func buildPricingFilters(f *pricingFlags) []types.Filter {
	var filters []types.Filter
	switch len(f.regions) {
	case 0:
	case 1:
		filters = append(filters, buildFilter("regionCode", types.FilterTypeEquals, f.regions[0]))
	default:
		filters = append(filters, buildFilter("regionCode", types.FilterTypeAnyOf, strings.Join(f.regions, ",")))
	}
	for _, group := range []struct {
		flagName   string
		filterType types.FilterType
		args       stringSlice
	}{
		{"--filter", types.FilterTypeEquals, f.filterEquals},
		{"--filter-any", types.FilterTypeAnyOf, f.filterAny},
		{"--filter-contains", types.FilterTypeContains, f.filterContains},
		{"--filter-none", types.FilterTypeNoneOf, f.filterNone},
	} {
		for _, arg := range group.args {
			field, value := splitKV(arg, group.flagName)
			filters = append(filters, buildFilter(field, group.filterType, value))
		}
	}
	return filters
}

// asMap type-asserts to a JSON object, returning nil for anything else.
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func parsePrice(v any) (float64, bool) {
	switch p := v.(type) {
	case string:
		f, err := strconv.ParseFloat(p, 64)
		return f, err == nil
	case float64:
		return p, true
	}
	return 0, false
}

// isFreeProduct reports whether every OnDemand price parses to 0; a missing
// OnDemand section or a non-numeric price means "not free".
func isFreeProduct(item map[string]any) bool {
	ondemand := asMap(asMap(item["terms"])["OnDemand"])
	if len(ondemand) == 0 {
		return false
	}
	for _, offer := range ondemand {
		for _, dim := range asMap(asMap(offer)["priceDimensions"]) {
			for _, price := range asMap(asMap(dim)["pricePerUnit"]) {
				p, ok := parsePrice(price)
				if !ok || p > 0 {
					return false
				}
			}
		}
	}
	return true
}

func retainKeys(m map[string]any, keep []string) {
	for k := range m {
		if !slices.Contains(keep, k) {
			delete(m, k)
		}
	}
}

// splitCSV turns a comma-delimited flag value into a list, nil when unset.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// postProcess parses one PriceList item and applies the serviceCode drop, the
// free filter, and the terms/attrs retention; nil means the item is dropped.
func postProcess(raw string, index int, f *pricingFlags) map[string]any {
	var item map[string]any
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		fail(fmt.Sprintf("invalid JSON in PriceList item %d: %v", index, err), "")
	}
	delete(item, "serviceCode")
	if f.excludeFree && isFreeProduct(item) {
		return nil
	}
	if keep := splitCSV(f.terms); keep != nil {
		if terms := asMap(item["terms"]); terms != nil {
			retainKeys(terms, keep)
		}
	}
	if keep := splitCSV(f.attrs); keep != nil {
		if attrs := asMap(asMap(item["product"])["attributes"]); attrs != nil {
			retainKeys(attrs, keep)
		}
	}
	return item
}

// marshalCompact renders compact JSON without HTML escaping, matching the raw
// API strings.
func marshalCompact(v any) string {
	var out strings.Builder
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fail(fmt.Sprintf("encoding output failed: %v", err), "")
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// enforceMaxChars refuses oversized output with a truncated sample and
// narrowing hints instead of flooding the caller.
func enforceMaxChars(output string, products []map[string]any, maxChars int) {
	if maxChars == -1 || len(output) <= maxChars {
		return
	}
	sample := []rune(marshalCompact(products[0]))
	if len(sample) > 600 {
		sample = sample[:600]
	}
	fmt.Fprintf(os.Stderr, "error: output is %d characters, over the --max-chars limit of %d\n",
		len(output), maxChars)
	fmt.Fprintf(os.Stderr, "first product (truncated): %s...\n", string(sample))
	fmt.Fprintln(os.Stderr, "hint: narrow instead of raising the cap — add --terms OnDemand, more --filter values, --attrs, or lower --max-results")
	os.Exit(1)
}

func pricingFlagSet(f *pricingFlags) *flag.FlagSet {
	fs := flag.NewFlagSet("pricing", flag.ExitOnError)
	fs.Var(&f.regions, "region", "Region code, repeatable; becomes a regionCode filter")
	fs.Var(&f.filterEquals, "filter", "EQUALS filter as FIELD=VALUE (repeatable)")
	fs.Var(&f.filterAny, "filter-any", "ANY_OF filter as FIELD=V1,V2 (repeatable)")
	fs.Var(&f.filterContains, "filter-contains", "CONTAINS filter as FIELD=SUBSTR (repeatable)")
	fs.Var(&f.filterNone, "filter-none", "NONE_OF filter as FIELD=V1,V2 (repeatable)")
	fs.StringVar(&f.terms, "terms", "", "Keep only these pricing term types, comma-separated (e.g. OnDemand,Reserved)")
	fs.StringVar(&f.attrs, "attrs", "", "Keep only these product attribute keys, comma-separated")
	fs.BoolVar(&f.excludeFree, "exclude-free", false, "Drop products whose every OnDemand price is 0")
	fs.IntVar(&f.maxResults, "max-results", 100, "Results per page, 1-100")
	fs.StringVar(&f.nextToken, "next-token", "", "Pagination token from a previous response")
	fs.IntVar(&f.maxChars, "max-chars", 100000, "Refuse output larger than this many characters; -1 = unlimited")
	return fs
}

func runPricing(ctx context.Context, args []string) {
	var f pricingFlags
	fs := pricingFlagSet(&f)
	serviceCode := requireServiceCode(parseArgs(fs, args), "pricing")

	var firstRegion string
	if len(f.regions) > 0 {
		firstRegion = f.regions[0]
	}
	client := makeClient(ctx, firstRegion)
	input := &pricing.GetProductsInput{
		ServiceCode: aws.String(serviceCode),
		Filters:     buildPricingFilters(&f),
		MaxResults:  aws.Int32(int32(min(max(f.maxResults, 1), 100))),
	}
	if f.nextToken != "" {
		input.NextToken = aws.String(f.nextToken)
	}
	resp, err := client.GetProducts(ctx, input)
	if err != nil {
		fail(fmt.Sprintf("GetProducts(%s) failed: %v", serviceCode, err),
			"verify the service code with `service-codes` and the filter fields with `service-attributes`")
	}

	var products []map[string]any
	for i, raw := range resp.PriceList {
		if item := postProcess(raw, i, &f); item != nil {
			products = append(products, item)
		}
	}
	if len(products) == 0 {
		fail(fmt.Sprintf("no products matched for %s", serviceCode),
			"check filter values with `attribute-values`; drop filters one at a time to isolate the wrong one")
	}

	result := struct {
		Products  []map[string]any `json:"products"`
		NextToken string           `json:"next_token,omitempty"`
	}{products, aws.ToString(resp.NextToken)}
	output := marshalCompact(result)
	enforceMaxChars(output, products, f.maxChars)
	fmt.Println(output)
}

func runPriceListUrls(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("price-list-urls", flag.ExitOnError)
	effectiveDate := fs.String("effective-date", "", "RFC3339 timestamp (default: now)")
	positionals := parseArgs(fs, args)
	if len(positionals) != 2 {
		fail("price-list-urls takes SERVICE_CODE and REGION", "")
	}
	serviceCode, region := positionals[0], positionals[1]

	date := time.Now()
	if *effectiveDate != "" {
		parsed, err := time.Parse(time.RFC3339, *effectiveDate)
		if err != nil {
			fail(fmt.Sprintf("invalid --effective-date %q: %v", *effectiveDate, err),
				"use RFC3339, e.g. 2026-07-01T00:00:00Z")
		}
		date = parsed
	}
	currency := "USD"
	if strings.HasPrefix(region, "cn-") {
		currency = "CNY"
	}

	client := makeClient(ctx, region)
	list, err := client.ListPriceLists(ctx, &pricing.ListPriceListsInput{
		ServiceCode:   aws.String(serviceCode),
		RegionCode:    aws.String(region),
		CurrencyCode:  aws.String(currency),
		EffectiveDate: aws.Time(date),
	})
	if err != nil {
		fail(fmt.Sprintf("ListPriceLists(%s, %s) failed: %v", serviceCode, region, err),
			"verify the service code with `service-codes` and the region with `attribute-values <code> regionCode`")
	}
	if len(list.PriceLists) == 0 {
		fail(fmt.Sprintf("no price list for %s in %s", serviceCode, region),
			"try another --effective-date, or verify the service/region pair")
	}
	priceList := list.PriceLists[0]
	arn := aws.ToString(priceList.PriceListArn)
	if arn == "" {
		fail("price list has no ARN", "")
	}

	urls := map[string]string{}
	for _, format := range priceList.FileFormats {
		resp, err := client.GetPriceListFileUrl(ctx, &pricing.GetPriceListFileUrlInput{
			PriceListArn: aws.String(arn),
			FileFormat:   aws.String(strings.ToUpper(format)),
		})
		if err != nil {
			fail(fmt.Sprintf("GetPriceListFileUrl(%s) failed: %v", format, err), "")
		}
		if aws.ToString(resp.Url) == "" {
			fail(fmt.Sprintf("AWS returned no URL for format %s", format), "")
		}
		urls[strings.ToLower(format)] = aws.ToString(resp.Url)
	}
	pretty, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		fail(fmt.Sprintf("encoding output failed: %v", err), "")
	}
	fmt.Println(string(pretty))
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: aws_pricing <command> [args]")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	fmt.Fprintln(os.Stderr, "  service-codes [--filter REGEX]                       List all service codes")
	fmt.Fprintln(os.Stderr, "  service-attributes SERVICE_CODE [--filter REGEX]     List a service's filterable attribute names")
	fmt.Fprintln(os.Stderr, "  attribute-values SERVICE_CODE ATTR... [--filter A=R] List attribute values")
	fmt.Fprintln(os.Stderr, "  pricing SERVICE_CODE [flags]                         Query product pricing as compact JSON")
	fmt.Fprintln(os.Stderr, "  price-list-urls SERVICE_CODE REGION [flags]          Bulk price-list download URLs")
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	commands := map[string]func(context.Context, []string){
		"service-codes":      runServiceCodes,
		"service-attributes": runServiceAttributes,
		"attribute-values":   runAttributeValues,
		"pricing":            runPricing,
		"price-list-urls":    runPriceListUrls,
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "aws_pricing: unknown command %q\n", args[0])
		usage()
		os.Exit(2)
	}
	cmd(context.Background(), args[1:])
}
