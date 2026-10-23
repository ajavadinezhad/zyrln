# Domestic domain list

`domains.txt` is bundled into the app when it is built. It is not downloaded while the app runs.

The routing code matches `.ir` domains and domains in this list, including subdomains. For example, `www.digikala.com` can match `digikala.com`.

To update it, review and replace the file using the `domains.txt` asset from [iran-hosted-domains releases](https://github.com/bootmortis/iran-hosted-domains/releases), then rebuild the apps. Review the diff before committing; a routing-list change affects which destinations bypass the relay.
