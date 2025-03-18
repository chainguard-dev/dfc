# dfc

<p align="center">
<img align="center" alt="dfc" width="250" src="./dfc.png">
</p>

<p align="center">
<b><code>d</code>ocker<code>f</code>ile <code>c</code>onverter</b>

<p align="center">CLI to convert Dockerfiles to use Chainguard Images and APKs in FROM and RUN lines</p>

</p>


---

## Installation

You can install `dfc` from Homebrew:

```sh
brew install chainguard-dev/tap/dfc
```

You can also install `dfc` from source:

```sh
go install github.com/chainguard-dev/dfc@latest
```

## Usage

Convert Dockerfile and print converted contents to terminal:

```sh
dfc ./Dockerfile
```

Save the output to new Dockerfile called `Dockerfile.chainguard`:

```sh
dfc ./Dockerfile > ./Dockerfile.chainguard
```

You can also pipe from stdin:

```sh
cat ./Dockerfile | dfc -
```

Convert the file in-place using `--in-place` / `-i` (saves backup in `.bak` file):

```sh
dfc --in-place ./Dockerfile
mv ./Dockerfile.bak ./Dockerfile # revert
```

Note: the `Dockerfile` and `Dockerfile.chainguard` in the root of this repo are not actually for building `dfc`, they
are symlinks to files in the [`testdata/`](./testdata/) folder so users can run the commands in this README.

## Configuration

### Chainguard org (cgr.dev namespace)

By default, FROM lines that have been mapped to Chainguard images will use "ORG" as a placeholder under `cgr.dev`:

```Dockerfile
FROM cgr.dev/ORG/<image>
```

To configure your `cgr.dev` namespace use the `--org` flag:

```
dfc --org="example.com" ./Dockerfile
```

Resulting in:

```Dockerfile
FROM cgr.dev/example.com/<image>
```

If mistakenly ran `dfc` with no configuration options and just want to replace the ORG
in the converted file, you can run something like this:

```sh
sed "s|/ORG/|/example.com/|" ./Dockerfile > dfc.tmp && mv dfc.tmp ./Dockerfile
```

### Alternate registry

To use an alternative registry domain and root namespace, use the `--registry` flag:

```
dfc --registry="r.example.com/cgr-mirror" ./Dockerfile
```

Resulting in:

```Dockerfile
FROM r.example.com/cgr-mirror/<image>
```

Note: the `--registry` flag takes precedence over the `--org` flag.

### Custom package mappings

If you need to use a modified version of the default, embedded package map
file [`packages.yaml`](./packages.yaml), use the `--mappings` flag:

```sh
dfc --mappings="./packages.yaml" ./Dockerfile
```

Want to submit an update to the default [`packages.yaml`](./packages.yaml)?
Please [open a GitHub pull request](https://github.com/chainguard-dev/dfc/compare).

## Examples

For complete before and after examples, see the [`testdata/`](./testdata/) folder.

### Convert a single `FROM` line

```sh
echo "FROM node" | dfc -
```

Result:

```Dockerfile
FROM cgr.dev/ORG/node:latest-dev
```

### Convert a single `RUN` line

```sh
echo "RUN apt-get update && apt-get install -y nano" | dfc -
```

Result:

```Dockerfile
RUN apk add -U nano
```

### Convert a whole Dockerfile

```sh
cat <<DOCKERFILE | dfc -
FROM node
RUN apt-get update && apt-get install -y nano
DOCKERFILE
```

Result:

```Dockerfile
FROM cgr.dev/ORG/node:latest-dev
USER root
RUN apk add -U nano
```

## Supported platforms

`dfc` detects the package manager being used and maps this to
a supported distro in order to properly convert RUN lines.
The following platforms are recognized:

| OS                           | Package manager            |
| ---------------------------- | -------------------------- |
| Alpine ("alpine")            | `apk`                      |
| Debian/Ubuntu ("debian")     | `apt-get` / `apt`          |
| Fedora/RedHat/UBI ("fedora") | `yum` / `dnf` / `microdnf` |


## How it works

### `FROM` line modifications

For each `FROM` line in the Dockerfile, `dfc` attempts to replace the base image with an equivalent Chainguard Image.

### `RUN` line modifications

For each `RUN` line in the Dockerfile, `dfc` attempts to detect the use of a known package manager (e.g. `apt-get` / `yum` / `apk`), extract the names of any packages being installed, try to map them via [`packages.yaml`](./packages.yaml), and replacing the old install with  `apk add -U <packages>`.

### `USER` line modifications

If `dfc` has detected the use of a package manager and ended up converting a RUN line,
then `USER root` will be appended under the last `FROM` line.

In the future we plan to handle this more elegantly, but this is the current state.

## JSON mode

Get converted Dockerfile as JSON using `--json` / `-j`:

```sh
dfc --json ./Dockerfile
```

Pipe it to `jq`:

```sh
dfc -j ./Dockerfile | jq
```

### Useful jq formulas

Reconstruct the Dockerfile pre-conversion:

```sh
dfc -j ./Dockerfile | jq -r '.lines[]|(.extra + .raw)'
```

Reconstruct the Dockerfile post-conversion:

```sh
dfc -j ./Dockerfile | jq -r '.lines[]|(.extra + (if .converted then .converted else .raw end))'
```

Convert and strip comments:

```sh
dfc -j ./Dockerfile | jq -r '.lines[]|(if .converted then .converted else .raw end)'
```

Get list of all distros detected from RUN lines:

```sh
dfc -j ./Dockerfile | jq -r '.lines[].run.distro' | grep -v null | sort -u
```

Get list of package managers detected from RUN lines:

```sh
dfc -j ./Dockerfile | jq -r '.lines[].run.manager' | grep -v null | sort -u
```

Get all the packages initially detected during parsing:

```sh
dfc -j ./Dockerfile | jq -r '.lines[].run.packages' | grep '"' | cut -d'"' -f 2 | sort -u | xargs
```

## Using from Go

The package `github.com/chainguard-dev/dfc/pkg/dfc` can be imported in Go and you can
parse and convert Dockerfiles on your own without the `dfc` CLI:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/chainguard-dev/dfc/pkg/dfc"
)

var (
	raw = []byte(strings.TrimSpace(`
		FROM node
		RUN apt-get update && apt-get install -y nano
	`))

	org = "example.com"
)

func main() {
	ctx := context.Background()

	// Parse the Dockefile bytes
	dockerfile, err := dfc.ParseDockerfile(ctx, raw)
	if err != nil {
		log.Fatalf("ParseDockerfile(): %v", err)
	}

	// Convert
	converted, err := dockerfile.Convert(ctx, dfc.Options{
		Organization: org,
	})
	if err != nil {
		log.Fatalf("dockerfile.Convert(): %v", err)
	}

	// Print converted Dockerfile content
	fmt.Println(converted)
}
```

## Limitations

- **Incomplete Conversion**: The tool makes a best effort to convert Dockerfiles but does not guarantee that the converted Dockerfiles will be buildable by Docker.
- **Comment and Spacing Preservation**: While the tool attempts to preserve comments and spacing, there may be cases where formatting is altered during conversion.
- **Dynamic Variables**: The tool may not handle dynamic variables in Dockerfiles correctly, especially if they are used in complex expressions.
- **Unsupported Directives**: Some Dockerfile directives may not be fully supported or converted, leading to potential build issues.
- **Package Manager Commands**: The tool focuses on converting package manager commands but may not cover all possible variations or custom commands.
- **Multi-stage Builds**: While the tool supports multi-stage builds, it may not handle all edge cases, particularly with complex stage dependencies.
- **Platform-Specific Features**: The tool may not account for platform-specific features or optimizations in Dockerfiles.
- **Security Considerations**: The tool does not perform security checks on the converted Dockerfiles, and users should review the output for potential vulnerabilities.

## Contact Us

For issues related strictly to `dfc` as an open source tool,
please [open a GitHub issue](https://github.com/chainguard-dev/dfc/issues/new?template=BLANK_ISSUE).

Chainguard customers: please share issues or feature requests with
your support contact so we can prioritize and escalate internally
(with or without a GitHub issue/PR).

Interested in Chainguard Images and want to get in touch with sales? Use [this form](https://www.chainguard.dev/contact).
