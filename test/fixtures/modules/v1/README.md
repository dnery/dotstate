# Module v1 Fixtures

Golden fixtures for the `dotstate.*.v1` module contract live under this tree.

Each case follows the layout from `docs/specs/modules.md`:

```text
<surface>/<case>/
  README.md
  input/
  desired/
  fact.golden.json
  plan.golden.json
  result.golden.json
  diagnostics.golden.json
  redaction.assert_absent.txt
```

A case only includes the golden files that apply to the behavior under test. Tests must also read `redaction.assert_absent.txt` and fail if any listed sentinel appears in serialized fixture output.
