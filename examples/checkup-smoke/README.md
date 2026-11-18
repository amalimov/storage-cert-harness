# Checkup smoke inputs

These files exercise the Checkup integration with two offline example tests. They do not run storage workloads or provide certification results. EX-006 emits its `sample_value` plan parameter as a synthetic measurement; its default is 4 and the bundled threshold is < 10. EX-003 emits a fixed 1200 MiB/s measurement. Both pass the bundled thresholds.

To change the EX-006 measurement, add this existing Harness plan override:

```yaml
overrides:
  EX-006:
    sample_value: 12
```

The Harness grader compares the resulting measurement with the EX-006 SLA from `thresholds.json`; the example adapter does not perform the comparison.

Run the example locally without cluster environment collection:

```sh
make build
./bin/harness run \
  --catalog examples/checkup-smoke/catalog.json \
  --plan examples/checkup-smoke/checkup-csi-smoke.yaml \
  --thresholds examples/checkup-smoke/thresholds.json \
  --skip-env --no-attestations --output /tmp/checkup-smoke-results
```

These files are development inputs. A real certification run requires its own catalog, thresholds, plan, and workload configuration.
