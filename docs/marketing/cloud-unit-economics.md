# mez Cloud: what it costs to run, and what it earns

Written 2026-10-11. Prices checked that day; INR at 88 to the dollar; GST
18% added where a vendor charges it. Change the numbers in "Assumptions"
and redo the arithmetic; the tables below were produced from them.

## The short version

- Running one venue on mez Cloud costs about **₹56 a month** in services
  (WhatsApp ₹22, R2 backup ₹25, hot-replica disk ₹9, email under ₹1).
  With three free months, budget **₹73 per paying venue**.
- Fixed cost for the first fifty venues is one Indian droplet plus
  compliance: about **₹6,500 a month**.
- At ₹699 (blended ₹630 with 60% on the annual plan) the service breaks
  even at **12 venues**. At ₹999 it breaks even at **8**.
- Gross margin settles at 75 to 84%. Cost is not the problem. Getting
  venues is the problem.

## Assumptions

| Item | Value | Source or reasoning |
|---|---|---|
| WhatsApp utility template, India | ₹0.115 + GST per message | Meta rate card as published by BSPs, unchanged 1 Oct 2026 |
| BSP platform fee | $0.001 per message (Gupshup self-serve) or 0 direct with Meta | Gupshup self-serve pricing |
| Messages per venue per month | 100 | 30 close-of-day reports, about 60 alerts, weekly summaries, a second recipient at some venues |
| R2 storage | $0.015 per GB-month, 1 GB per venue | Cloudflare R2 rate card |
| R2 Class A operations | $4.50 per million; 60,000 per venue per month | Litestream syncing every 30 s across a 14-hour day plus compaction |
| Hot-replica block storage | $0.10 per GB-month, 1 GB per venue | DigitalOcean volumes |
| Email | $0.10 per 1,000, 10 per venue | Amazon SES |
| Trial overhead | 1.3x variable cost | Three free months means roughly 0.3 unpaid venues per paying one at steady state |
| Payment collection | 2.8% of revenue | Razorpay 2% + GST on the fee, applied to a GST-inclusive invoice |
| Annual share | 60% of venues on the annual plan | Indian SMB software is sold annually; Petpooja sells annually |
| Overhead | ₹3,500 a month | CA, GST filing, company compliance, domain, averaged |
| Support hire | ₹25,000 a month | From 300 venues |
| Engineer hire | ₹90,000 a month | From 1,000 venues |

Infrastructure is DigitalOcean Bangalore: 4 GB $24, 8 GB $48, 16 GB $96,
managed Postgres $15 and $60, load balancer $12, backups at 20% of the
droplet. Hetzner has no Indian region; Singapore is about the same price
with less transfer included.

## The one number that can ruin this

Litestream's default is to sync the WAL **every second**. A busy cafe at
that setting produces about 1.5 million R2 Class A operations a month,
which is **₹594 per venue per month**, nearly the whole subscription.
At a 30-second interval it is ₹24. The cloud build must set the sync
interval and part size deliberately, and the first ADR of Phase 14
should say so. A one-minute recovery point is fine for a cafe.

## Price ₹699 a month, ₹6,999 a year (blended ₹630)

| Venues | Revenue | Gateway | Variable | Infra | Tools+overhead | Infra-only profit | Margin | Staff | Profit after staff |
|---|---|---|---|---|---|---|---|---|---|
| 10 | 6,296 | 176 | 732 | 2,552 | 3,800 | -965 | -15% | 0 | -965 |
| 20 | 12,591 | 353 | 1,465 | 2,552 | 3,800 | 4,422 | 35% | 0 | 4,422 |
| 50 | 31,477 | 881 | 3,662 | 5,104 | 3,800 | 18,030 | 57% | 0 | 18,030 |
| 100 | 62,955 | 1,763 | 7,324 | 11,088 | 4,000 | 38,780 | 62% | 0 | 38,780 |
| 300 | 188,865 | 5,288 | 21,972 | 27,720 | 4,500 | 129,385 | 69% | 25,000 | 104,385 |
| 500 | 314,775 | 8,814 | 36,620 | 31,416 | 5,000 | 232,926 | 74% | 25,000 | 207,926 |
| 1,000 | 629,550 | 17,627 | 73,239 | 61,776 | 6,500 | 470,407 | 75% | 140,000 | 330,407 |
| 5,000 | 3,147,750 | 88,137 | 366,197 | 220,000 | 13,500 | 2,459,916 | 78% | 305,000 | 2,154,916 |
| 10,000 | 6,295,500 | 176,274 | 732,394 | 423,104 | 23,500 | 4,940,228 | 78% | 610,000 | 4,330,228 |

Break-even with one box and no staff: 12 venues. Covering one support hire: 58.

## Price ₹999 a month, ₹9,999 a year (blended ₹900)

| Venues | Revenue | Gateway | Variable | Infra | Tools+overhead | Infra-only profit | Margin | Staff | Profit after staff |
|---|---|---|---|---|---|---|---|---|---|
| 10 | 8,996 | 252 | 732 | 2,552 | 3,800 | 1,659 | 18% | 0 | 1,659 |
| 20 | 17,991 | 504 | 1,465 | 2,552 | 3,800 | 9,670 | 54% | 0 | 9,670 |
| 50 | 44,978 | 1,259 | 3,662 | 5,104 | 3,800 | 31,152 | 69% | 0 | 31,152 |
| 100 | 89,955 | 2,519 | 7,324 | 11,088 | 4,000 | 65,024 | 72% | 0 | 65,024 |
| 300 | 269,865 | 7,556 | 21,972 | 27,720 | 4,500 | 208,117 | 77% | 25,000 | 183,117 |
| 500 | 449,775 | 12,594 | 36,620 | 31,416 | 5,000 | 364,146 | 81% | 25,000 | 339,146 |
| 1,000 | 899,550 | 25,187 | 73,239 | 61,776 | 6,500 | 732,847 | 81% | 140,000 | 592,847 |
| 5,000 | 4,497,750 | 125,937 | 366,197 | 220,000 | 13,500 | 3,772,116 | 84% | 305,000 | 3,467,116 |
| 10,000 | 8,995,500 | 251,874 | 732,394 | 423,104 | 23,500 | 7,564,628 | 84% | 610,000 | 6,954,628 |

Break-even with one box and no staff: 8 venues. Covering one support hire: 39.

Setup fees (₹2,500 to ₹5,000 once per venue) are not in the tables. They
pay for the founder's day at the venue and should not be waived.

## Vendors, and why

| Need | Use | Why | Not |
|---|---|---|---|
| Relay + cloud compute | DigitalOcean Bangalore | Indian region, flat pricing, 4 TB transfer included, snapshots | Hetzner (no India), AWS Mumbai (about 3x for the same box), Hostinger KVM (cheapest, but weaker API and support) |
| Edge, DNS, TLS, DDoS | Cloudflare free plan | Already in use for the relay; wildcard venue subdomains | Paid plans until a real need |
| Backup object storage | Cloudflare R2 | No egress fee, S3 API Litestream already speaks, 10 GB and 1M ops free | AWS S3 (egress), Backblaze B2 (cheaper storage, but ops and egress add up with Litestream) |
| WhatsApp | Meta Cloud API direct for the first hundreds of venues; Gupshup self-serve if the approval path is a problem | Direct has no platform fee; Gupshup adds $0.001 a message and handles onboarding | AiSensy, Interakt, Wati: ₹1,000 to ₹3,800 a month platform fees and a per-message markup aimed at marketing blasts |
| Email | Amazon SES | $0.10 per 1,000 | Resend ($20 a month minimum once past 3,000) |
| Payments | Razorpay Subscriptions with UPI Autopay and cards | Indian, autopay mandates, 2% | Stripe India (invite-only, weaker UPI) |
| Monitoring | Grafana Cloud free tier, Uptime Kuma on the relay box | Free at this scale | Datadog |
| Errors | Sentry free tier (5k events) | Free at this scale | |
| Support inbox | Zoho Mail free, WhatsApp Business app | Free | Google Workspace until there is a team |

## What the numbers say about the price

- Cost per venue is ₹73 either way. Price is a conversion question, not a
  cost question.
- ₹999 a month is ₹11,988 a year: Petpooja's territory. ₹6,999 a year
  reads as "seven thousand" against their "ten to twelve". Both are
  below the incumbent.
- Recommendation: list ₹999 a month and ₹9,999 a year. Offer the first
  100 venues ₹699 and ₹6,999 locked for as long as they stay. Early
  venues are using an alpha and giving feedback; that is what the
  discount pays for, and it is honest urgency.

## Is it worth doing

- At 10 to 20 venues this is a hobby that pays for its servers.
- At 100 venues it is one salary, about ₹39,000 to ₹65,000 a month.
- At 300 to 500 venues it is a real small business, ₹1 to ₹3.4 lakh a
  month after one hire.
- At 1,000 venues it is a company: ₹75 lakh to ₹1.08 crore a year in
  revenue with a team of three.
- Petpooja crossed 100,000 outlets. 1,000 is one percent of that, in a
  country with over a million cafes and small restaurants.

The economics are good because the venue does the expensive work on its
own computer. The risk is entirely in distribution: each venue needs a
visit, a printer that works and an owner who trusts a new name.
