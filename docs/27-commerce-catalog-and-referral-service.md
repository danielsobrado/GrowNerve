# 27 — Commerce Catalog, Compatibility, and Referral Service

## Purpose

GrowNerve can monetize hardware and consumable recommendations without turning the farm application into an advertising platform.

The commercial model should help answer a useful question:

> What should I buy, and which products will actually work with the farm I am building or operating?

The architecture deliberately separates **technical component truth** from **commercial offers**.

```text
GrowNerve component/domain model
            |
            | neutral technical identity and requirements
            v
   compatibility / product mapping
            |
            v
 GrowNerve Commerce Service
       /         |          \
      v          v           v
  Amazon     AliExpress    vendor/direct
  or other      or local      offers
  merchant      merchant
```

The core GrowNerve application remains fully functional if the commerce service is disabled, unreachable, or removed entirely.

This document extends:

- `24-component-plugin-system.md` — reusable component identity and registry
- `26-component-taxonomy-and-capabilities.md` — categories, subtypes, capabilities, properties, channels, ports, and anchors

The commerce service is **not implemented today**.

## Core principles

1. **Commerce is optional.** Farm management, telemetry, alerts, control, the digital twin, import/export, and local-first operation never depend on the commerce service.
2. **Technical definitions stay neutral.** Component JSON contains no affiliate URL, referral tag, commission rate, sponsored rank, or merchant credential.
3. **Commercial secrets stay off the client.** Merchant API credentials, affiliate account configuration, referral templates, and signing secrets live only in the commerce service.
4. **Compatibility comes before monetization.** A product must be technically compatible before it can be recommended as compatible.
5. **Commission does not influence organic ranking.** Organic recommendations are ranked from user value and technical suitability, not payout.
6. **Sponsored placement is a separate lane.** Sponsored products are visibly labeled and never disguised as the best technical result.
7. **Explain recommendations.** The user should be able to see why a product fits: dimensions, voltage, connectors, power, capacity, control interface, or other relevant facts.
8. **Disclose material relationships.** Affiliate and sponsored relationships are displayed clearly and close to the recommendation/link.
9. **Merchant policy is part of correctness.** Link generation, redirects, price display, images, caching, and attribution follow the rules of each merchant/affiliate program.
10. **Minimize farm-data disclosure.** The commerce service receives only the technical/market context needed to return useful recommendations.
11. **No dark patterns.** No forced interstitials, fake scarcity, hidden sponsorship, automatic merchant opening, or commerce actions that block normal farm workflows.
12. **User choice wins.** Self-hosted and browser-only users can disable all network commerce calls.

## Why use a separate commerce server

A separate deployed service is the correct boundary for monetization.

### Keep credentials out of open-source clients

The public React bundle and component packs must never contain:

```text
merchant API credentials
affiliate network secrets
vendor portal credentials
webhook secrets
signing keys
private commission configuration
```

Even referral tags that are not secret should be centrally managed so they can change without releasing a new GrowNerve build.

### One place for regional logic

The same product may have different offers in:

```text
United States
UAE
Philippines
Spain / EU
other supported markets
```

The service can choose the correct merchant storefront, currency, program account, product variant, and disclosure policy for the requested market.

### Merchant APIs and quotas belong on the server

Merchant catalog APIs often require credentials, rate limiting, retry/backoff, caching, and program-specific usage rules. The browser should not implement those integrations.

### Commercial rules can evolve independently

GrowNerve can update:

- merchant mappings
- referral tags
- offer availability
- sponsorship campaigns
- product verification
- price freshness rules
- compliance text

without changing the local farm data model or Three.js component packs.

### Static GitHub Pages can still monetize safely

Browser-only GrowNerve can call the public read side of the commerce service over HTTPS while keeping IndexedDB farm data local. If commerce is disabled or the network is offline, those UI sections simply disappear or show cached non-authoritative data.

## Deployment boundary

Recommended first deployment:

```text
                     GrowNerve application
                 browser or full server UI
                           |
             optional HTTPS commerce calls
                           |
                           v
              commerce.grownerve.<domain>
                GrowNerve Commerce API
              /          |             \
             v           v              v
        product DB   connector jobs   click/conversion
             |           |              events
             |           v
             |      merchant APIs/feeds
             |
             v
      compatibility catalog
```

Optionally, a second hostname may exist for programs that explicitly permit tracked redirects:

```text
go.grownerve.<domain>
```

Do **not** assume every affiliate program permits redirect/cloaking. The commerce service must support direct merchant links as the default and enable server redirects only per program policy.

### Recommended implementation stack

Keep this service small:

```text
Go HTTP service
PostgreSQL
scheduled connector/import jobs
structured logging/metrics
no MQTT
no access to GrowNerve farm PostgreSQL
```

A separate repository such as `GrowNerve-Commerce` can be created when implementation starts. The contracts remain documented here because the main application is the consumer.

The commerce service is intentionally **not** part of the GrowNerve control plane.

## Trust boundary

The commerce service must never become authoritative for:

- device state
- telemetry
- command safety
- automation
- alerts
- grow records
- component technical definitions already pinned in a farm archive

It has no database credentials for the main GrowNerve runtime and no MQTT credentials.

A commerce outage can at worst remove shopping/recommendation information.

It must never stop a light, fan, pump, alert, import, export, or farm edit from working.

## Core domain model

### `CatalogProduct`

A neutral real-world product identity.

Conceptual fields:

```text
product_id
manufacturer
brand
model
product_family
product_type
canonical_name
manufacturer_url
status
verified_status
technical_attributes
```

Examples:

```text
Spider Farmer SF2000
AC Infinity CLOUDLINE T6
Atlas Scientific EZO-pH
DFRobot EC sensor
```

A catalog product is not an offer and contains no referral ranking.

### `ProductVariant`

Represents a concrete variant where compatibility or purchasing differs:

```text
voltage
plug type
size
power rating
connector
color where technically relevant
pack quantity
regional SKU
```

Example:

```text
same fan family
  -> 120 V US variant
  -> 230 V EU variant
```

Do not recommend the wrong electrical variant merely because the product family name matches.

### `Merchant`

Represents the seller/storefront, for example:

```text
Amazon US
Amazon UAE
AliExpress seller/store
manufacturer direct
regional hydroponics distributor
local retailer
```

### `AffiliateProgram`

Program-specific commercial configuration kept server-side:

```text
program_id
merchant_id
markets
link_mode
credential reference
referral/account configuration
required disclosure policy
price/content policy
status
```

`link_mode` is policy-driven, for example:

```text
direct_affiliate_url
permitted_server_redirect
coupon_or_referral_code
non_affiliate_link
```

There is no universal redirect rule.

### `Offer`

A purchasable product/variant from a merchant.

Conceptual fields:

```text
offer_id
product_variant_id
merchant_id
market
currency
price (only when program terms permit display)
availability
shipping summary when authorized
merchant product identifier
destination/referral representation
affiliate boolean
sponsored boolean (normally false; sponsorship is modeled separately)
checked_at
expires_at
source
```

Offers are mutable/fresh data. Component definitions are immutable technical revisions. Do not mix the two lifecycles.

### `ComponentProductMapping`

Maps a GrowNerve reusable component revision or technical requirement to real products.

Two mapping forms are useful.

#### Exact product mapping

Vendor-specific component:

```text
com.vendor.product.model
        -> exact CatalogProduct/ProductVariant
```

#### Compatibility mapping

Generic component:

```text
grownerve.light.panel.generic
        -> product candidates satisfying requirements
```

This allows a generic scene component to recommend several real products without changing the scene definition.

### `CompatibilityProfile`

Normalized requirements used to compare a farm/component need with a product.

Examples:

```text
category/subtype
physical dimensions
electrical voltage/frequency
maximum current/power
control method
connector/port types
pipe/hose diameter
mounting constraints
flow requirement
reservoir capacity
light output/coverage requirement
sensor measurement dimension/range
IP/environment rating
region/plug requirement
```

### `Recommendation`

A computed recommendation is ephemeral and explanatory.

Conceptual result:

```json
{
  "recommendation_id": "...",
  "type": "compatible",
  "product_id": "...",
  "compatibility": {
    "compatible": true,
    "score": 0.94,
    "confidence": "verified",
    "reasons": [
      "supports 230 V / 50 Hz",
      "fits the selected tent footprint",
      "supports required dimming range"
    ],
    "warnings": []
  },
  "offers": []
}
```

The explanation is part of the product, not optional decoration.

### `SponsoredPlacement`

Sponsorship is deliberately separate from recommendation truth.

Conceptual fields:

```text
campaign_id
product_id
market
placement
starts_at
ends_at
label
budget/contract metadata (private)
```

A sponsored product still has to pass any compatibility gate required by the placement. Paying GrowNerve cannot make an incompatible 120 V device appear compatible with a 230 V farm.

### `DisclosurePolicy`

Represents text/placement requirements by merchant/program/market.

The UI should receive normalized disclosure data such as:

```text
relationship = affiliate | sponsored | none
short disclosure
required site/global disclosure identifier
merchant-specific notice
policy version
```

Do not rely on one hidden legal page.

## Recommendation classes

GrowNerve should support explicit recommendation intent rather than one generic "recommended" flag.

```text
required      missing hardware needed for a configured system
compatible    products that satisfy the current need
alternative   equivalent alternatives to an existing product
upgrade       technically better optional replacement
replacement   same role for failed/aging hardware
consumable    nutrients, seeds, calibration solution, filters, media, tubing, etc.
maintenance   replacement/cleaning/service item
expansion     optional new capability such as EC monitoring
bundle        multiple products that complete a known subsystem
```

These types allow the UI to distinguish operational need from commercial opportunity.

## Compatibility before ranking

Recommendation is two distinct steps.

```text
candidate products
      |
      v
hard compatibility filter
      |
      v
suitability scoring
      |
      v
organic ranking
      |
      +-------------------+
      |                   |
      v                   v
organic results      sponsored lane
```

### Hard compatibility

A failure should exclude a candidate when relevant:

```text
wrong voltage/frequency
wrong connector
outside required physical envelope
insufficient load capacity
insufficient pump head/flow
unsupported control interface
wrong pipe diameter where adapters are not acceptable
unsupported sensor measurement/range
unsafe environment/IP rating
wrong regional variant
```

### Suitability score

Potential normalized factors:

```text
technical fit
quality of technical data
verified manufacturer mapping
size/fit quality
performance headroom
energy efficiency
maintainability
availability in market
price/value when comparable data is authorized
shipping suitability when available
user preferences
```

### Commission exclusion

Commission amount, EPC, bounty, or affiliate payout is **not an input to organic compatibility or organic ranking**.

If GrowNerve wishes to sell promotion, it appears in a separately labeled sponsored slot.

This rule should be enforced in code and covered by regression tests, not left as a policy promise.

## Recommendation explanations

Do not show unexplained percentages only.

Good:

```text
Recommended for your 3 × 3 tent

✓ 230 V / 50 Hz variant
✓ fits the selected mounting area
✓ within the 200–300 W lighting target
✓ dimmable
✓ compatible with your existing switched outlet

Affiliate offer available from Vendor X
```

For an uncertain result:

```text
Potentially compatible

✓ electrical requirements match
✓ size fits
? exact tent-pole clamp diameter is not verified
```

Unknown is better than invented certainty.

## Application surfaces

Commerce should be contextual and useful rather than interruptive.

### Missing-component guidance

Example:

```text
Your DWC layout has no aeration source.

Suggested requirement:
- air pump
- two air stones
- suitable tubing

View compatible hardware
```

### Component inspector

For a generic or existing component:

```text
Generic 240 W LED Panel

[technical information]

Compatible products
3 products available in your market
```

### Build / shopping list

A farm design can derive a bill-of-material-like list:

```text
1 × grow tent
1 × LED panel
1 × circulation fan
1 × reservoir
1 × air pump
2 × air stones
1 × controller
sensors...
```

The user can mark items as:

```text
already owned
need to buy
ordered
installed
not required
```

Commerce offers attach only to items needing purchase.

### Maintenance/replacement

Example:

```text
pH calibration solution is low
View compatible replacement
```

or:

```text
Air stone service due
Replacement options
```

### Expansion suggestions

Example:

```text
Your reservoir has temperature and level monitoring.
EC monitoring is not installed.

Why it may help
[technical explanation]

View compatible EC sensors
```

Do not disguise optional upselling as a safety requirement.

### Dedicated catalog

A browse/search catalog can exist later, but contextual recommendations are more valuable than building a generic storefront first.

## Product and offer API

The public commerce API is read-oriented and safe for browser use.

Possible endpoints:

```text
GET  /v1/products/{product_id}
GET  /v1/products/{product_id}/offers?market=AE
GET  /v1/components/{component_id}/products
POST /v1/recommendations
GET  /v1/offers/{offer_id}/link
GET  /v1/disclosures/current?market=AE
```

Merchant ingestion/vendor administration uses a separate authenticated administrative surface and is never exposed as anonymous browser API.

### Recommendation request

The client sends only normalized requirements needed for the recommendation.

Conceptual request:

```json
{
  "market": "AE",
  "currency": "AED",
  "need": {
    "category": "lighting",
    "subtype": "led_panel",
    "requirements": {
      "voltage_v": 230,
      "frequency_hz": 50,
      "target_power_w_min": 200,
      "target_power_w_max": 300,
      "maximum_width_m": 0.85
    }
  }
}
```

Do not send the whole `FarmData` document.

### Recommendation response

Conceptual response:

```json
{
  "generated_at": "...",
  "market": "AE",
  "recommendations": [
    {
      "product_id": "...",
      "name": "...",
      "compatibility": {
        "compatible": true,
        "score": 0.94,
        "reasons": ["..."],
        "warnings": []
      },
      "offers": [
        {
          "offer_id": "...",
          "merchant": "...",
          "affiliate": true,
          "price": null,
          "currency": "AED",
          "checked_at": "...",
          "expires_at": "..."
        }
      ]
    }
  ],
  "disclosure": {
    "relationship": "affiliate",
    "text": "GrowNerve may earn a commission if you buy through these links."
  }
}
```

The client still asks for the actual current link when the user chooses an offer. This allows tags and link formats to rotate centrally.

## Affiliate-link delivery

The user specifically choosing a merchant offer is the point at which a referral URL should be resolved.

```text
user clicks merchant offer
      |
      v
GET /v1/offers/{id}/link
      |
      v
commerce service applies current program/market config
      |
      v
client receives link response
      |
      v
browser navigates because of the user's explicit action
```

The client never concatenates referral codes.

### Link response

Conceptual response:

```json
{
  "offer_id": "...",
  "mode": "direct_affiliate_url",
  "url": "https://merchant.example/...",
  "affiliate": true,
  "disclosure": "GrowNerve may earn a commission from this purchase.",
  "expires_at": "..."
}
```

### Direct links versus redirect links

Do not universally hide merchant links behind `go.grownerve...`.

Some affiliate programs restrict automatic or indirect redirects. For example, current Amazon Associates guidance states that visitors may not be automatically redirected to Amazon and its program policies restrict indirect redirecting links without the required affirmative user action.

Therefore the server supports program-specific modes:

```text
direct_affiliate_url       return the approved merchant Special Link to the client
permitted_server_redirect  return/use GrowNerve redirect only when the program permits it
coupon_or_referral_code    return merchant URL + visible code where applicable
non_affiliate_link         ordinary merchant/manufacturer link
```

Merchant policy configuration, not application convenience, decides the mode.

Never implement an open redirect endpoint such as:

```text
/go?url=<arbitrary-user-url>
```

A permitted redirect resolves only an existing server-side `offer_id` to an allow-listed HTTPS destination.

## Price, availability, images, and merchant content

Do not scrape retailers as the default product-data strategy.

Use, in priority order:

```text
official merchant/affiliate API
manufacturer feed/API
contracted distributor feed
manually verified catalog entry
```

Each merchant connector owns its terms for:

- price display
- price caching
- availability
- product images
- ratings/reviews
- trademarks
- refresh intervals
- attribution links

The normalized service response should make freshness explicit:

```text
checked_at
expires_at
source
price_display_allowed
content_attribution
```

If GrowNerve cannot legally/reliably display a current price, show:

```text
Check current price at Merchant X
```

rather than a stale or invented price.

Do not import retailer ratings/reviews unless the applicable API/license explicitly permits their display.

## Disclosures and commercial transparency

Disclosure is a UI requirement, not only a legal-document requirement.

### Affiliate relationship

Use clear language near the recommendation/link, for example:

```text
GrowNerve may earn a commission if you buy through this link. This does not change your price.
```

The exact final text is jurisdiction/program dependent and must be reviewed before production.

Current FTC guidance says affiliate relationships should be disclosed clearly and conspicuously and that proximity to the recommendation/link matters. The FTC specifically warns that a bare phrase such as "affiliate link" may not adequately communicate that the publisher earns money.

Reference:

- <https://www.ftc.gov/business-guidance/resources/ftcs-endorsement-guides-what-people-are-asking>

### Amazon-specific example

Current Amazon Associates guidance requires link-level disclosure and also requires the site identification statement:

```text
As an Amazon Associate I earn from qualifying purchases.
```

Reference:

- <https://affiliate-program.amazon.com/help/node/topic/GHQNZAU6669EZS98>

Merchant-specific obligations can change. Store them as versioned program/compliance configuration and review them periodically rather than assuming this document is permanent legal advice.

### Sponsored results

Always label them visibly:

```text
Sponsored
```

or clearer jurisdiction-appropriate wording.

Do not use subtle color alone to communicate sponsorship.

### Free products / vendor relationships

If GrowNerve or its maintainers received free hardware, paid testing, discounts, or other material consideration relevant to an endorsement, the content/recommendation system needs a disclosure mechanism for that relationship as well.

## Merchant neutrality and ranking policy

Organic results should be auditable.

The recommendation engine should emit a scoring explanation sufficient to answer:

```text
Why is Product A above Product B?
```

Private commercial fields such as:

```text
commission rate
expected payout
campaign budget
vendor contract value
```

must not be available to the organic scoring function.

A clean implementation uses separate data structures/processes so accidental coupling is difficult.

Conceptually:

```text
TechnicalRecommendationEngine
    inputs: technical/user-value fields only

SponsoredPlacementEngine
    inputs: compatible candidates + campaign rules

UIComposer
    renders organic recommendations
    renders separately labeled sponsored placement
```

## User-value ranking

For organic offers of the same compatible product, useful ranking inputs may include:

```text
market availability
correct regional variant
current authorized price
shipping information when available
seller/manufacturer verification
warranty/support
return policy metadata when reliably sourced
freshness of offer data
user merchant preference
```

Do not use affiliate payout to sort merchants.

## Vendor and manufacturer ecosystem

Later, manufacturers can publish official product/component data.

An official mapping may provide:

```text
manufacturer identity
verified product identity
component revision mapping
exact dimensions/specifications
GLB asset
technical documents
supported regions
merchant/distributor links
```

GrowNerve can show:

```text
Manufacturer verified
```

only after the vendor identity and submitted data are actually verified.

Potential monetization products later:

```text
verified manufacturer account
hosted official component pack
vendor analytics
lead/referral reporting
sponsored placement
featured launch placement
regional distributor mapping
```

Payment never grants the ability to override GrowNerve compatibility validation.

## Consumables and recurring commerce

Recurring consumables can be more valuable than one-time hardware.

Examples:

```text
seeds
nutrients
pH calibration solutions
EC calibration solutions
filters
air stones
tubing
growing media
net pots
replacement probes
cleaning supplies
```

Recommendations should come from actual state when possible:

```text
inventory low
calibration due
maintenance due
replacement interval reached
new grow planned
```

Do not fabricate urgency when there is no operational reason.

## Privacy model

The commerce service should not need personal farm records.

### Send only what is necessary

Allowed examples:

```text
country/market
currency
component ID or generic requirement
voltage/frequency
physical size constraint
required capability
port/connector requirement
optional user price preference
```

Do not send by default:

```text
facility name
farm UUIDs
user identity
exact address
telemetry history
grow observations
camera/media
command history
inventory history unrelated to the recommendation
```

### Region

Prefer an explicit user-selected commerce market or coarse country code.

Do not require precise geolocation merely to choose a storefront.

### Analytics

The minimum useful aggregate analytics are:

```text
recommendation impressions
merchant-offer clicks
product/category
market
campaign/sponsorship ID when applicable
conversion summary received from affiliate network
```

Persistent user-level tracking is not required for the initial business model.

If analytics/cookies/local identifiers require consent in a jurisdiction, GrowNerve must gate them accordingly. The product should still resolve a purchase link without behavioral profiling.

### IP/log retention

Normal infrastructure logs should use bounded retention and avoid turning IP addresses into a long-term user profile.

## Click and conversion attribution

### Clicks

For direct-link programs, the client can request the current link immediately after the user's click. The commerce service records an aggregate click-resolution event and returns the merchant URL.

Do not generate fake clicks by prefetching the link endpoint as if the user clicked it.

If a merchant program counts link-generation requests differently from actual clicks, model those events separately.

### Conversions

Affiliate networks may provide reports, APIs, or postbacks.

Normalize conversions at the commerce service:

```text
program
merchant
product/offer when known
market
order/conversion reference hash where permitted
revenue/commission
occurred_at
```

Do not copy merchant customer identity into GrowNerve.

Conversion data is business analytics, not farm history.

## Security

### Merchant connector isolation

Connectors make outbound requests only to configured merchant/API hosts.

Protect against:

```text
SSRF
credential leakage
unbounded response bodies
malicious feed URLs
HTML/script injection
unexpected redirects
API quota exhaustion
```

### Public API

Use:

```text
HTTPS only
strict CORS for known production origins where appropriate
rate limiting
bounded request sizes
schema validation
safe text/URL normalization
short timeouts
structured errors
```

Product HTML from vendors/merchants is not trusted. Prefer structured plain-text fields and sanitized supported media URLs/content.

### Link safety

Only HTTPS merchant destinations from approved program configuration are returned.

A merchant or community component pack cannot inject an arbitrary affiliate destination into the commerce service.

### Administration

Catalog editing, affiliate credentials, campaigns, and vendor verification require strong authenticated admin/vendor APIs separate from the anonymous read API.

## Availability and failure behavior

Commerce is non-critical.

Application behavior on failure:

```text
commerce timeout
    -> hide/disable current offers
    -> optionally show cached product identity with stale label
    -> continue normal GrowNerve operation
```

Do not retry commerce calls aggressively from every scene render.

Use short timeouts and cached/query-deduplicated requests.

### Offline/browser-only mode

Possible settings:

```text
commerce.enabled = true | false
commerce.base_url = <configured service>
```

Static/offline users can disable commerce completely.

A previously cached offer is never presented as current without its freshness timestamp/expiry state.

## Application adapter

Keep commerce out of domain control code.

Conceptual frontend interface:

```ts
interface CommerceClient {
  recommendations(request: RecommendationRequest): Promise<RecommendationResult>;
  offers(productId: string, market: string): Promise<Offer[]>;
  resolveLink(offerId: string): Promise<ResolvedOfferLink>;
}
```

Screens/inspectors depend on this interface.

The implementation may be:

```text
RemoteCommerceClient -> commerce.grownerve...
DisabledCommerceClient -> returns no offers
```

The normal `FarmRepository` does not become responsible for commerce.

## Component-pack rules

Component packs may include neutral product identity hints such as:

```text
manufacturer
model
manufacturer product code
```

when those are factual and appropriate for that definition.

They may **not** include authoritative commercial fields such as:

```text
affiliate URL
referral tag
commission rate
sponsored priority
merchant API key
tracking script
```

A third-party component pack cannot monetize GrowNerve users merely by embedding its own tracking link.

Community/vendor submissions can propose a product mapping, but the commerce catalog controls whether that mapping is trusted/verified.

## Shopping-list model

A useful later abstraction is a local farm shopping list separate from offers.

Conceptually:

```text
ShoppingNeed
  id
  source/component requirement
  quantity
  status: needed | owned | ordered | installed | skipped
  notes
```

A shopping need belongs to the user's farm data.

Offers remain remote/transient commerce data.

Do not persist an affiliate URL as the authoritative representation of what the farm needs.

## Business models enabled

The architecture supports several revenue streams without breaking the open/local product.

### Affiliate commissions

Commission when a user intentionally buys through a disclosed tracked link.

### Vendor referral / lead programs

Direct manufacturer/distributor referral programs where appropriate.

### Sponsored placements

Clearly separated sponsored slots for compatible products.

### Verified manufacturer program

Paid vendor tooling/verification/official packs while technical validation remains independent.

### Hosted services later

Separate from commerce:

```text
hosted GrowNerve
remote access
team/collaboration
managed backups
advanced analytics/AI
```

The free local application does not need to be intentionally crippled to make commerce viable.

## First commercial implementation

Keep the initial proof deliberately simple.

### C0 — Contract and trust boundary

Deliver:

- this architecture
- `Product`, `Offer`, `ComponentProductMapping`, `Recommendation`, and disclosure schemas
- documented organic-ranking policy
- program-specific link-mode model
- privacy contract

### C1 — Standalone commerce server

Deliver:

- separate Go service and PostgreSQL database
- manually curated small product catalog
- merchant/program configuration through secrets/admin data
- public product/offer/recommendation API
- `DisabledCommerceClient` and `RemoteCommerceClient`
- no access to farm/control databases

Start with manually curated offers. Do not integrate five affiliate networks before validating that users actually click recommendations.

### C2 — Pilot recommendations

Cover only the real reference system:

```text
3 × 3 tent
200–300 W LED class
circulation fan
air pump / air stones
30 L-class reservoir
temperature/RH/water sensors
ESP32/control hardware
basic consumables
```

Exit criteria:

- recommendations are technically explainable
- user market changes the offers/variant appropriately
- commerce service outage has no effect on farm operation
- disclosures render next to affiliate links

### C3 — Merchant connectors and freshness

Only after C2 proves useful:

- official merchant/affiliate APIs
- scheduled imports
- caching and quota control
- region-specific catalog/variants
- policy-compliant price/availability display

### C4 — Attribution and reporting

Deliver:

- aggregate click-resolution analytics
- conversion import/postbacks where programs provide them
- revenue reporting by merchant/product/category/market
- privacy/retention controls

### C5 — Vendor ecosystem

Later:

- vendor onboarding
- verified product/component mappings
- official pack hosting
- sponsored campaigns
- vendor analytics

## Testing

### Compatibility tests

- wrong electrical variant rejected
- dimension mismatch rejected
- missing required interface produces warning/exclusion
- exact vendor mapping resolves exact product
- generic component resolves compatible candidates
- unknown technical data never becomes false certainty

### Ranking tests

- affiliate commission changes do not alter organic rank
- sponsorship does not alter organic rank
- incompatible sponsored product cannot appear in compatibility-gated placement
- organic scoring explanation is deterministic for the same input/catalog revision

### Link tests

- client never constructs referral code itself
- disabled/expired offer does not resolve
- direct-link programs return approved direct links
- redirect mode works only for programs configured to permit it
- no arbitrary/open redirect
- no automatic merchant navigation without user action

### Disclosure tests

- affiliate disclosure visible adjacent to actionable merchant link
- sponsored label visible before interaction
- merchant-required global/site disclosure rendered where configured
- no relationship is labeled affiliate when it is genuinely non-affiliate

### Privacy tests

- recommendation request excludes farm UUIDs and telemetry
- commerce API works without user identity
- analytics path is separable from recommendation correctness

### Failure tests

- DNS/service timeout does not break the inspector or twin
- stale cached offer is labeled stale or hidden
- merchant connector outage does not erase last known catalog product identity

## Acceptance criteria

The commerce architecture is successful when:

- GrowNerve remains completely useful with commerce disabled
- affiliate credentials/referral configuration are absent from the browser bundle and component packs
- a generic component can map to several real purchasable products without changing its technical definition
- recommendations explain compatibility using real technical properties
- the wrong regional/electrical variant is rejected
- organic ranking is independent of affiliate payout
- sponsored results are separately labeled
- affiliate relationships are disclosed clearly near actionable links
- merchant link/redirect rules are respected per program
- the service can rotate affiliate configuration without releasing a new GrowNerve version
- the service receives only minimal recommendation context, not full farm data
- no commerce request can issue a command or modify farm state
- a commerce outage cannot affect crop-control operation

## Non-goals for the first release

Do not build these before C1/C2 proves useful:

- a full ecommerce checkout owned by GrowNerve
- inventory/fulfillment/logistics
- payment processing
- a required marketplace
- real-time scraping of retailers
- behavioral ad targeting
- cross-site tracking profiles
- bidding that changes organic recommendation order
- automatic merchant redirects
- dozens of merchant integrations
- complex ML ranking when deterministic compatibility rules are sufficient

GrowNerve's commercial advantage should come from trustworthy compatibility and context, not from behaving like an ad network.