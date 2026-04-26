# Demo

`terminal-demo.sh` is the recording source for the short SetupProof terminal
demo, and `terminal-demo.txt` is the checked transcript. The script builds the
CLI from this repository, creates a temporary Git project, and shows the
maintainer path:

1. find candidate quickstarts with `suggest`;
2. list explicitly marked blocks;
3. review execution semantics without running commands;
4. run the marked block in a copied workspace.

Run it directly:

```sh
docs/demo/terminal-demo.sh
```

The default pacing is intended for a 12-20 second terminal recording on a
typical development machine. For a fast smoke check, remove the pacing:

```sh
SETUPPROOF_DEMO_PAUSE=0 docs/demo/terminal-demo.sh
```
