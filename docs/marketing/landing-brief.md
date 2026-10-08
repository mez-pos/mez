# mez landing page: brief for Claude Design

Paste everything below the line into Claude Design, with the logo kit
attached. Replace the placeholders in square brackets first.

---

## What you are designing

A marketing landing page for **mez**, an open-source point of sale for
cafes, bars and small restaurants. One page, scrolling, responsive
(desktop first, but it must read well on a phone, since cafe owners will
open the link from WhatsApp). Output: a single HTML page I can host
on Cloudflare Pages, or a Figma-style design I can hand to a developer.

The product runs as one small program on the venue's own computer. The
till keeps selling with no internet. Guests can order from a QR code on
the table. The code is open source (AGPL) and free to self-host; we sell
a paid "Cloud" plan around it.

## Audience

Primary: an owner-operator of one to three cafes in India. Runs a
Windows laptop at the counter. Pays Petpooja or Posist around 10,000 to
25,000 rupees a year, or still runs on paper. Technical enough to run an
installer, not technical enough to run Docker. Reads the page on a phone
in a spare minute between customers.

Secondary: a self-hoster or developer who found the GitHub repo and
wants to know if this is serious.

Write for the first reader. Keep the developer content to one section
near the bottom.

## Voice

Plain, confident, short sentences. Say what it does, not what it
"empowers". No startup jargon, no exclamation marks, no emoji. Prices
and claims are concrete. The product name is always lowercase: mez.

Do not use em dashes anywhere in the copy. Use commas, full stops or
colons instead.

## Visual language (match the product, do not invent a new one)

The product looks like a beautifully printed menu brought to life.
Use the attached logo kit for the mark and wordmark. Use these tokens
exactly; do not add colours outside this list.

| Role | Hex |
|---|---|
| Page ground (parchment) | #F8F3E9 |
| Cards, header (ivory) | #FFFDF8 |
| Sunken wells, inputs | #F2EADB |
| Headings (maroon ink) | #5B1F1F |
| Body text | #3A2E2A |
| Secondary text | #5F5850 |
| Muted text (13px minimum) | #7A6E66 |
| Hairline borders | #EAE0D0 |
| Strong hairline (inputs, secondary buttons) | #DCCDB4 |
| Primary action (forest green) | #2F4A2C |
| Primary hover | #24391F |
| Green tint (selected, success fills) | #E8EFE3 |
| Attention (terracotta): links, secondary CTA, active nav | #B9522F |
| Terracotta tint (soft badges) | #F6E4DC |
| Highlight (mustard): small accents only, never text on light ground | #E7A83A |
| Dark section ground ("espresso night") | #241C17 |
| Dark section raised card | #332821 |
| Text on dark | #F5EADB |

Rules:
- Serif for headings and display prices (something in the family of
  Fraunces, Playfair or Source Serif). A neutral sans (Inter or similar)
  for everything functional.
- Shadows are low and warm brown, never grey. Corners are round and soft.
- Green commits, terracotta asks for attention. One primary button per
  section.
- Use the dark "espresso night" ground for exactly one section (the
  kitchen display or the "how it works" diagram), so the page has one
  moment of contrast.
- Status pills: pastel fill, saturated dot, one word.
- Generous whitespace. This should feel like a good menu, not a SaaS
  template.

## Page structure, top to bottom

1. **Header.** Logo left. Links: Features, Pricing, Docs, GitHub.
   A small terracotta "Get mez Cloud" button right.

2. **Hero.** Headline: "The till that never stops selling." Sub-line:
   "An open-source point of sale for cafes and small restaurants. Runs
   on your own computer, works with no internet, and the data stays in
   your shop." Two buttons: green "Download for Windows" (primary),
   terracotta outline "See Cloud plans". Below the buttons, one line in
   muted text: "Free and open source. Linux and Docker builds on GitHub."
   Right side or below: a composed screenshot of the register screen on
   a tablet, with the guest ordering screen on a phone overlapping it.
   [I will provide screenshots; leave placeholders at 4:3 for the tablet
   and 9:19.5 for the phone.]

3. **Three pains, three answers.** Three cards, each with a short
   serif heading and two sentences.
   - "The internet drops. The queue does not." mez sells, prints and
     keeps the Z-report straight with no connection. Online ordering
     pauses, the counter does not.
   - "Your numbers belong to you." Every sale is in a database on your
     computer. Export it any time. Leave any time.
   - "No per-device fees." Run it on the laptop you already have. Add
     tills and a kitchen screen without paying per screen.

4. **Features.** A grid of eight, each with a small line icon, a
   heading and one sentence:
   - Register and split bills
   - Kitchen display with timers and recall
   - Floor plan and tables
   - GST invoices and credit notes
   - Shifts, drawer counts and the Z-report
   - Inventory, recipes and wastage
   - Guests order from a QR on the table, pay by UPI or card
   - Bring your menu from a spreadsheet in minutes

5. **Guest ordering, as a story.** Dark "espresso night" section. A
   simple diagram, left to right: guest's phone, our relay in the cloud,
   the computer in your cafe, the kitchen screen. Caption: "The relay
   only passes guest orders through. Nothing on your network is opened
   to the internet, and the relay keeps nothing." Phone screenshot
   placeholder of the guest menu.

6. **Pricing.** Three columns on parchment, the middle one raised as
   the recommended plan.

   | | Self-hosted | Cloud | Cloud Plus |
   |---|---|---|---|
   | Price | Free forever | 699 rupees a month, or 6,999 a year | 1,499 rupees a month |
   | For | You run everything yourself | One venue that wants it handled | Venues that want a person on call |
   | Includes | Every feature. Run your own relay from the docs | Hosted relay and `yourcafe.mez.app` address, QR ordering, off-site backup, remote back office from your phone, update alerts, email support | Everything in Cloud, plus WhatsApp support, help setting up printers and the payment gateway, multiple venues |
   | Button | Download (green outline) | Start free for 3 months (green) | Talk to us (terracotta outline) |

   Under the table, one line: "Setup at your venue, including printer,
   drawer and table QR sheets: 2,500 to 5,000 rupees once." And:
   "Prices are per venue, not per device." [Adjust prices if I change
   them.]

7. **Switching is a spreadsheet.** Short section. Heading: "Bring
   your menu in ten minutes." Copy: download the template, fill it in
   Excel or Google Sheets, import. Items, choices, ingredients and
   recipes. Show the import dialog screenshot [placeholder] with its
   "12 items to add, 2 example rows left out" preview.

8. **Hardware.** One row of four: any ESC/POS network receipt printer,
   cash drawer through the printer, Windows 10 or 11 or Linux, any
   tablet or phone with a browser for tills and the kitchen screen.

9. **Open source.** Light section with a GitHub card: stars, licence
   (AGPL-3.0), "Read the code", "Run your own relay". One paragraph:
   "If we disappear tomorrow, your till still works, and anyone can
   maintain it. That is the point of open source for a business tool."

10. **Testimonial.** One quote from [friend's cafe name], with a photo
    placeholder of the counter. [I will provide the quote.]

11. **Final call.** Heading: "Try it on the laptop you already have."
    Green "Download for Windows", terracotta text link "or get Cloud".
    Muted line: "Installs in under ten minutes. No card needed."

12. **Footer.** Logo, Docs, GitHub, Pricing, Privacy, Contact on
    WhatsApp [number], "Made in India".

## Things to avoid

- No stock photos of smiling baristas. Use product screenshots and
  the diagram.
- No gradients, no glassmorphism, no purple.
- No "AI-powered", no "revolutionary", no "seamless".
- No countdown timers or fake urgency.
- Do not shrink body text below 15px on mobile.

## Deliverables

- The page at 1440px and 390px widths.
- The hero and pricing sections as separate frames I can reuse for
  social posts.
- A short note listing the fonts used and where each screenshot
  placeholder is, so I can drop in real images.
