# Demo

`setupproof.gif` is the short terminal demo used by the main README.
`setupproof.tape` is the VHS source, and `prepare-recording.sh` prepares the
temporary project used by that recording.

The GIF shows the maintainer path:

1. inspect the marked README quickstart;
2. review execution semantics without running commands;
3. run the marked block in a copied workspace.

Regenerate the GIF:

```sh
go run github.com/charmbracelet/vhs@v0.11.0 docs/demo/setupproof.tape
```

The VHS path requires `git`, `go`, `bash`, `ttyd`, and `ffmpeg`.

`terminal-demo.sh` is a plain terminal transcript source, and
`terminal-demo.txt` is the checked transcript. The script builds the CLI from
this repository, creates a temporary Git project, and shows a longer maintainer
path:

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
