# Connect an existing domain to Cloudflare

Read this only for a domain not yet using active Cloudflare DNS. Fetch the
current [official full-setup instructions](https://developers.cloudflare.com/dns/zone-setups/full-setup/setup/)
and any registrar-specific instructions linked there before guiding changes.
Use current provider UI/CLI behavior, not remembered button labels or timers.

1. Establish the registrar, current DNS provider and whether the domain serves
   a website or email. Explain that full setup changes DNS hosting; it does not
   require transferring domain registration. Find the matching official guide
   for the user's registrar when its steps are needed.
2. Help the user sign in to/create their Cloudflare account and add their own
   domain. Offer the available free setup where suitable; choosing a paid plan
   requires the user's explicit choice. Obtain direction before account/zone
   changes. Discover CLI commands before using them if performing this through cf.
3. Have the user export/review the current DNS records, then compare the imported
   records in Cloudflare. Automatic discovery is incomplete. Preserve website
   and subdomain records, MX, SPF, DKIM, DMARC, service/verification records and
   provider-required proxy settings. Do not claim a public DNS scan found every
   record. Pause for the user's review if the inventory is incomplete.
4. Follow the official DNSSEC migration instructions for their current state.
   A stale registrar DS record can break resolution after a nameserver change.
   Explain the required change and obtain direction; do not disable DNSSEC or
   alter registrar settings silently.
5. Show the exact assigned Cloudflare nameservers and affected domain. Obtain
   explicit direction before replacing registrar nameservers. Guide the user
   through registrar access, or perform the authorized steps where supported.
6. Verify public NS delegation and Cloudflare zone status, plus the existing
   website/email records. Report propagation as pending when appropriate and
   give a resumable next step. Re-enable/configure DNSSEC through the documented
   procedure after activation when it was part of this migration.

Completion: the user's zone is active and correctly delegated, with existing
services accounted for. Then return to the wizard's prerequisites. This migration
is separate from the later rdr-owned wildcard/TXT operations; use the official
provider migration process here and rundevrun's backend for project-domain setup.
