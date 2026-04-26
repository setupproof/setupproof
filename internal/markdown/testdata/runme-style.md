# Compatibility Fixture

```sh { name=setup cwd=. }
echo "runme metadata is not a SetupProof marker"
```

```sh {"id":"test","name":"run"}
echo "json metadata is not a SetupProof marker"
```

```sh setupproof id=marked
echo "selected"
```
