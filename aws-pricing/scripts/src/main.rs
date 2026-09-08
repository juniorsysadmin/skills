//! CLI helper for the aws-pricing skill (see ../SKILL.md).
//!
//! Five subcommands over the AWS Price List Query API:
//! service-codes, service-attributes, attribute-values, pricing, price-list-urls.

use aws_config::{BehaviorVersion, Region};
use aws_sdk_pricing::error::DisplayErrorContext;
use aws_sdk_pricing::types::{Filter, FilterType};
use aws_smithy_types::date_time::Format;
use aws_smithy_types::DateTime;
use clap::{Parser, Subcommand};
use regex::{Regex, RegexBuilder};
use serde_json::{json, Map, Value};

// The Price List API exists only in these regions; anything else maps to the
// nearest one by prefix.
const PRICING_REGIONS: [&str; 5] = [
    "us-east-1",
    "eu-central-1",
    "ap-southeast-1",
    "ap-south-1",
    "cn-northwest-1",
];
const ANY_OF_VALUE_LIMIT: usize = 1024;

#[derive(Parser)]
#[command(
    name = "aws_pricing",
    about = "Query the AWS Price List API",
    version
)]
struct Cli {
    #[command(subcommand)]
    cmd: Cmd,
}

#[derive(Subcommand)]
enum Cmd {
    /// List all service codes, one per line
    ServiceCodes {
        /// Case-insensitive regex to filter codes
        #[arg(long)]
        filter: Option<String>,
    },
    /// List the filterable attribute names of a service
    ServiceAttributes {
        service_code: String,
        /// Case-insensitive regex to filter names
        #[arg(long)]
        filter: Option<String>,
    },
    /// List the values of one or more attributes of a service
    AttributeValues {
        service_code: String,
        #[arg(required = true)]
        attributes: Vec<String>,
        /// Case-insensitive regex per attribute, repeatable
        #[arg(long = "filter", value_name = "ATTR=REGEX")]
        filters: Vec<String>,
    },
    /// Query product pricing; compact JSON on stdout
    Pricing {
        service_code: String,
        /// Region code, repeatable; becomes a regionCode filter
        #[arg(long)]
        region: Vec<String>,
        /// EQUALS filter, repeatable
        #[arg(long = "filter", value_name = "FIELD=VALUE")]
        filter_equals: Vec<String>,
        /// ANY_OF filter, repeatable; values comma-separated
        #[arg(long = "filter-any", value_name = "FIELD=V1,V2")]
        filter_any: Vec<String>,
        /// CONTAINS filter, repeatable
        #[arg(long = "filter-contains", value_name = "FIELD=SUBSTR")]
        filter_contains: Vec<String>,
        /// NONE_OF filter, repeatable; values comma-separated
        #[arg(long = "filter-none", value_name = "FIELD=V1,V2")]
        filter_none: Vec<String>,
        /// Keep only these pricing term types (e.g. OnDemand,Reserved)
        #[arg(long, value_delimiter = ',')]
        terms: Option<Vec<String>>,
        /// Keep only these product attribute keys
        #[arg(long, value_delimiter = ',')]
        attrs: Option<Vec<String>>,
        /// Drop products whose every OnDemand price is 0
        #[arg(long)]
        exclude_free: bool,
        /// Results per page, 1-100
        #[arg(long, default_value_t = 100)]
        max_results: i32,
        /// Pagination token from a previous response
        #[arg(long)]
        next_token: Option<String>,
        /// Refuse output larger than this many characters; -1 = unlimited
        #[arg(long, default_value_t = 100_000)]
        max_chars: i64,
    },
    /// Bulk price-list download URLs for a service in a region
    PriceListUrls {
        service_code: String,
        region: String,
        /// RFC3339 timestamp (default: now)
        #[arg(long)]
        effective_date: Option<String>,
    },
}

fn fail(msg: &str, hint: Option<&str>) -> ! {
    eprintln!("error: {msg}");
    let lower = msg.to_lowercase();
    if lower.contains("credential") || lower.contains("expiredtoken") || lower.contains("token has expired") {
        eprintln!("hint: no valid AWS credentials — check AWS_PROFILE or run `aws sso login`");
    }
    if let Some(h) = hint {
        eprintln!("hint: {h}");
    }
    std::process::exit(1);
}

fn compile_regex(pattern: &str) -> Regex {
    RegexBuilder::new(pattern)
        .case_insensitive(true)
        .build()
        .unwrap_or_else(|e| fail(&format!("invalid regex {pattern:?}: {e}"), None))
}

fn split_kv<'a>(arg: &'a str, flag: &str) -> (&'a str, &'a str) {
    arg.split_once('=').unwrap_or_else(|| {
        fail(
            &format!("{flag} needs FIELD=VALUE, got {arg:?}"),
            None,
        )
    })
}

/// Map any region to the nearest region that hosts the Price List API.
fn pricing_region(requested: Option<&str>) -> String {
    let Some(r) = requested else {
        return "us-east-1".to_string();
    };
    if PRICING_REGIONS.contains(&r) {
        return r.to_string();
    }
    if r.starts_with("cn-") {
        "cn-northwest-1"
    } else if r.starts_with("eu-") || r.starts_with("me-") || r.starts_with("af-") {
        "eu-central-1"
    } else if r.starts_with("ap-") {
        "ap-south-1"
    } else {
        "us-east-1"
    }
    .to_string()
}

/// Client pinned to a pricing region; the ambient AWS region never applies.
async fn make_client(requested_region: Option<&str>) -> aws_sdk_pricing::Client {
    let config = aws_config::defaults(BehaviorVersion::latest())
        .region(Region::new(pricing_region(requested_region)))
        .load()
        .await;
    aws_sdk_pricing::Client::new(&config)
}

async fn service_codes(filter: Option<String>) {
    let client = make_client(None).await;
    let re = filter.as_deref().map(compile_regex);
    let mut codes: Vec<String> = Vec::new();
    let mut pages = client.describe_services().into_paginator().send();
    while let Some(page) = pages.next().await {
        let page = page.unwrap_or_else(|e| {
            fail(&format!("DescribeServices failed: {}", DisplayErrorContext(&e)), None)
        });
        for service in page.services() {
            let code = service.service_code();
            if re.as_ref().is_none_or(|re| re.is_match(code)) {
                codes.push(code.to_string());
            }
        }
    }
    codes.sort();
    codes.dedup();
    if codes.is_empty() {
        fail(
            "no service codes matched",
            Some("loosen or drop --filter; run without --filter to see every code"),
        );
    }
    println!("{}", codes.join("\n"));
}

async fn service_attributes(service_code: String, filter: Option<String>) {
    let client = make_client(None).await;
    let re = filter.as_deref().map(compile_regex);
    let resp = client
        .describe_services()
        .service_code(&service_code)
        .send()
        .await
        .unwrap_or_else(|e| {
            fail(
                &format!("DescribeServices({service_code}) failed: {}", DisplayErrorContext(&e)),
                Some("verify the service code with `service-codes`"),
            )
        });
    let Some(service) = resp.services().first() else {
        fail(
            &format!("unknown service code {service_code:?}"),
            Some("verify the service code with `service-codes`"),
        );
    };
    let mut names: Vec<String> = service
        .attribute_names()
        .iter()
        .filter(|n| re.as_ref().is_none_or(|re| re.is_match(n)))
        .cloned()
        .collect();
    names.sort();
    if names.is_empty() {
        fail(
            &format!("no attributes matched for {service_code}"),
            Some("loosen or drop --filter"),
        );
    }
    println!("{}", names.join("\n"));
}

async fn attribute_values(service_code: String, attributes: Vec<String>, filters: Vec<String>) {
    let client = make_client(None).await;
    let filter_map: Vec<(String, Regex)> = filters
        .iter()
        .map(|f| {
            let (attr, pattern) = split_kv(f, "--filter");
            (attr.to_string(), compile_regex(pattern))
        })
        .collect();
    // Buffer everything so a failure on any attribute produces no output at all.
    let mut out = String::new();
    for attr in &attributes {
        let re = filter_map.iter().find(|(a, _)| a == attr).map(|(_, re)| re);
        let mut values: Vec<String> = Vec::new();
        let mut pages = client
            .get_attribute_values()
            .service_code(&service_code)
            .attribute_name(attr)
            .max_results(100)
            .into_paginator()
            .send();
        while let Some(page) = pages.next().await {
            let page = page.unwrap_or_else(|e| {
                fail(
                    &format!(
                        "GetAttributeValues({service_code}, {attr}) failed: {}",
                        DisplayErrorContext(&e)
                    ),
                    Some("verify the names with `service-codes` and `service-attributes`"),
                )
            });
            for v in page.attribute_values() {
                if let Some(value) = v.value() {
                    if re.is_none_or(|re| re.is_match(value)) {
                        values.push(value.to_string());
                    }
                }
            }
        }
        values.sort();
        values.dedup();
        out.push_str(&format!("# {attr}\n"));
        for v in &values {
            out.push_str(v);
            out.push('\n');
        }
    }
    print!("{out}");
}

fn build_filter(field: &str, filter_type: FilterType, value: &str) -> Filter {
    if matches!(filter_type, FilterType::AnyOf | FilterType::NoneOf)
        && value.len() > ANY_OF_VALUE_LIMIT
    {
        fail(
            &format!(
                "{field} value list is {} characters; AWS caps ANY_OF/NONE_OF at {ANY_OF_VALUE_LIMIT}",
                value.len()
            ),
            Some("split the query into several calls with shorter value lists"),
        );
    }
    Filter::builder()
        .field(field)
        .r#type(filter_type)
        .value(value)
        .build()
        .unwrap_or_else(|e| fail(&format!("invalid filter {field}={value}: {e}"), None))
}

/// True when every OnDemand price parses to 0. A missing OnDemand section or a
/// non-numeric price means "not free".
fn is_free_product(item: &Value) -> bool {
    let Some(ondemand) = item.pointer("/terms/OnDemand").and_then(Value::as_object) else {
        return false;
    };
    if ondemand.is_empty() {
        return false;
    }
    for offer in ondemand.values() {
        let Some(dims) = offer.get("priceDimensions").and_then(Value::as_object) else {
            continue;
        };
        for dim in dims.values() {
            let Some(prices) = dim.get("pricePerUnit").and_then(Value::as_object) else {
                continue;
            };
            for price in prices.values() {
                let parsed = match price {
                    Value::String(s) => s.parse::<f64>().ok(),
                    Value::Number(n) => n.as_f64(),
                    _ => None,
                };
                match parsed {
                    Some(p) if p > 0.0 => return false,
                    Some(_) => {}
                    None => return false,
                }
            }
        }
    }
    true
}

#[allow(clippy::too_many_arguments)]
async fn pricing(
    service_code: String,
    region: Vec<String>,
    filter_equals: Vec<String>,
    filter_any: Vec<String>,
    filter_contains: Vec<String>,
    filter_none: Vec<String>,
    terms: Option<Vec<String>>,
    attrs: Option<Vec<String>>,
    exclude_free: bool,
    max_results: i32,
    next_token: Option<String>,
    max_chars: i64,
) {
    let client = make_client(region.first().map(String::as_str)).await;

    let mut filters: Vec<Filter> = Vec::new();
    match region.len() {
        0 => {}
        1 => filters.push(build_filter("regionCode", FilterType::Equals, &region[0])),
        _ => filters.push(build_filter("regionCode", FilterType::AnyOf, &region.join(","))),
    }
    for (flag, filter_type, args) in [
        ("--filter", FilterType::Equals, &filter_equals),
        ("--filter-any", FilterType::AnyOf, &filter_any),
        ("--filter-contains", FilterType::Contains, &filter_contains),
        ("--filter-none", FilterType::NoneOf, &filter_none),
    ] {
        for arg in args {
            let (field, value) = split_kv(arg, flag);
            filters.push(build_filter(field, filter_type.clone(), value));
        }
    }

    let resp = client
        .get_products()
        .service_code(&service_code)
        .set_filters(Some(filters))
        .max_results(max_results.clamp(1, 100))
        .set_next_token(next_token)
        .send()
        .await
        .unwrap_or_else(|e| {
            fail(
                &format!("GetProducts({service_code}) failed: {}", DisplayErrorContext(&e)),
                Some("verify the service code with `service-codes` and the filter fields with `service-attributes`"),
            )
        });

    let mut products: Vec<Value> = Vec::new();
    for (i, raw) in resp.price_list().iter().enumerate() {
        let mut item: Value = serde_json::from_str(raw).unwrap_or_else(|e| {
            fail(&format!("invalid JSON in PriceList item {i}: {e}"), None)
        });
        if let Some(obj) = item.as_object_mut() {
            obj.remove("serviceCode");
        }
        if exclude_free && is_free_product(&item) {
            continue;
        }
        if let Some(keep) = &terms {
            if let Some(t) = item.get_mut("terms").and_then(Value::as_object_mut) {
                t.retain(|k, _| keep.iter().any(|w| w == k));
            }
        }
        if let Some(keep) = &attrs {
            if let Some(a) = item
                .pointer_mut("/product/attributes")
                .and_then(Value::as_object_mut)
            {
                a.retain(|k, _| keep.iter().any(|w| w == k));
            }
        }
        products.push(item);
    }

    if products.is_empty() {
        fail(
            &format!("no products matched for {service_code}"),
            Some("check filter values with `attribute-values`; drop filters one at a time to isolate the wrong one"),
        );
    }

    let mut result = json!({ "products": products });
    if let Some(token) = resp.next_token() {
        result["next_token"] = json!(token);
    }
    let output = result.to_string();

    if max_chars != -1 && output.len() as i64 > max_chars {
        let sample: String = result["products"][0].to_string().chars().take(600).collect();
        eprintln!(
            "error: output is {} characters, over the --max-chars limit of {max_chars}",
            output.len()
        );
        eprintln!("first product (truncated): {sample}...");
        eprintln!("hint: narrow instead of raising the cap — add --terms OnDemand, more --filter values, --attrs, or lower --max-results");
        std::process::exit(1);
    }
    println!("{output}");
}

async fn price_list_urls(service_code: String, region: String, effective_date: Option<String>) {
    let client = make_client(Some(&region)).await;
    let date = match &effective_date {
        Some(s) => DateTime::from_str(s, Format::DateTime).unwrap_or_else(|e| {
            fail(
                &format!("invalid --effective-date {s:?}: {e}"),
                Some("use RFC3339, e.g. 2026-07-01T00:00:00Z"),
            )
        }),
        None => DateTime::from(std::time::SystemTime::now()),
    };
    let currency = if region.starts_with("cn-") { "CNY" } else { "USD" };

    let list = client
        .list_price_lists()
        .service_code(&service_code)
        .region_code(&region)
        .currency_code(currency)
        .effective_date(date)
        .send()
        .await
        .unwrap_or_else(|e| {
            fail(
                &format!(
                    "ListPriceLists({service_code}, {region}) failed: {}",
                    DisplayErrorContext(&e)
                ),
                Some("verify the service code with `service-codes` and the region with `attribute-values <code> regionCode`"),
            )
        });
    let Some(price_list) = list.price_lists().first() else {
        fail(
            &format!("no price list for {service_code} in {region}"),
            Some("try another --effective-date, or verify the service/region pair"),
        );
    };
    let Some(arn) = price_list.price_list_arn() else {
        fail("price list has no ARN", None);
    };

    let mut urls = Map::new();
    for format in price_list.file_formats() {
        let resp = client
            .get_price_list_file_url()
            .price_list_arn(arn)
            .file_format(format.to_uppercase())
            .send()
            .await
            .unwrap_or_else(|e| {
                fail(
                    &format!("GetPriceListFileUrl({format}) failed: {}", DisplayErrorContext(&e)),
                    None,
                )
            });
        let Some(url) = resp.url() else {
            fail(&format!("AWS returned no URL for format {format}"), None);
        };
        urls.insert(format.to_lowercase(), json!(url));
    }
    println!("{}", serde_json::to_string_pretty(&Value::Object(urls)).unwrap());
}

#[tokio::main(flavor = "current_thread")]
async fn main() {
    match Cli::parse().cmd {
        Cmd::ServiceCodes { filter } => service_codes(filter).await,
        Cmd::ServiceAttributes {
            service_code,
            filter,
        } => service_attributes(service_code, filter).await,
        Cmd::AttributeValues {
            service_code,
            attributes,
            filters,
        } => attribute_values(service_code, attributes, filters).await,
        Cmd::Pricing {
            service_code,
            region,
            filter_equals,
            filter_any,
            filter_contains,
            filter_none,
            terms,
            attrs,
            exclude_free,
            max_results,
            next_token,
            max_chars,
        } => {
            pricing(
                service_code,
                region,
                filter_equals,
                filter_any,
                filter_contains,
                filter_none,
                terms,
                attrs,
                exclude_free,
                max_results,
                next_token,
                max_chars,
            )
            .await
        }
        Cmd::PriceListUrls {
            service_code,
            region,
            effective_date,
        } => price_list_urls(service_code, region, effective_date).await,
    }
}
