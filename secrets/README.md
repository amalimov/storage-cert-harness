# Legacy secrets directory

SLA threshold encryption described in ADR-0004 was superseded by ADR-0017.
Real SLA thresholds are public and are maintained in the repository's
`thresholds.json`; this directory is not used to store threshold files.

Do not commit credentials or other sensitive values here. Backend credentials
must use the references and handling described in ADR-0005. The ignored
`secrets/plain/` path remains only to avoid tracking any legacy local files.
