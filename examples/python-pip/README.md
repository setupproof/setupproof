# Python/pip Example

This example has no third-party dependencies. The `--no-index` flag makes the
pip step fail instead of using a package index if a dependency is later added.

<!-- setupproof id=python-pip-test -->
```sh
python3 -m venv .venv
.venv/bin/python -m pip install --no-index --requirement requirements.txt
.venv/bin/python -m unittest discover -s tests
```
