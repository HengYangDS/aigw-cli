package ci

// pipeline.cue owns CI topology. Forge files are generated projections.

#OperatingSystem: "darwin" | "linux" | "windows"
#JobID:           "source-and-governance" | "native-darwin" | "native-linux" | "native-windows" | "release-readiness" | "package" | "publish" | "release"

#Job: {
	stage: "verify" | "package" | "publish" | "release"
	rank:  int & >=0
	needs: [...#JobID]
}

commands: {
	bootstrap: #"""
		version=$(awk '$1 == "min_version" {gsub(/"/, "", $3); print $3}' mise.toml)
		test -n "$version"
		case "$(uname -m)" in
		  x86_64|amd64)
		    arch=x64
		    checksum=cfe49784ec9683b38510846958cfecd9b59da84d4e8a38d18ffda19dc2941ead
		    ;;
		  aarch64|arm64)
		    arch=arm64
		    checksum=b92744ceb9a01f0bb198bfcf2ba49c36918c9e4353a34be50f23d5b6e93c28ee
		    ;;
		  *) echo "unsupported Linux architecture: $(uname -m)" >&2; exit 1 ;;
		esac
		asset="mise-linux-$arch.tar.gz"
		package_url="$CI_API_V4_URL/projects/$CI_PROJECT_ID/packages/generic/ci-mise/$version/$asset"
		tmpdir=$(mktemp -d)
		trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM
		curl --fail --silent --show-error --location \
		  --connect-timeout 5 --max-time 120 \
		  --header "JOB-TOKEN: $CI_JOB_TOKEN" \
		  "$package_url" --output "$tmpdir/$asset"
		(
		  cd "$tmpdir"
		  printf '%s  %s\n' "$checksum" "$asset" | sha256sum --check --strict
		  tar --extract --gzip --file "$asset"
		)
		install -m 0755 "$tmpdir/mise/bin/mise" /usr/local/bin/mise
		test "$(mise --version | awk '{print $1}')" = "$version"
		"""#
	install: "mise install --locked"
	source:  "mise exec --locked -- go run ./tools/ci source"
	native: {
		for platform in ["darwin", "linux", "windows"] {
			(platform): "mise exec --locked -- go run ./tools/ci native --platform \(platform)"
		}
	}
	build:   "mise exec --locked -- go run ./tools/release build-ci build/release dist"
	upload:  "mise exec --locked -- go run ./tools/release upload-gitlab dist"
	publish: "mise exec --locked -- go run ./tools/release publish-gitlab dist"
}

nativeToolchain: MISE_ENABLE_TOOLS: "go,cue"

// This map owns native execution evidence only. Product release targets remain
// solely owned by .config/release/goreleaser.yaml.
nativeEvidence: {
	darwin: {
		name: "macOS"
		gitlab: tags: ["$AIGW_GITLAB_DARWIN_RUNNER_TAG"]
		github: runner: "macos-latest"
	}
	linux: {
		name: "Linux"
		gitlab: tags: ["$AIGW_GITLAB_LINUX_RUNNER_TAG"]
		github: runner: "ubuntu-latest"
	}
	windows: {
		name: "Windows"
		gitlab: tags: ["$AIGW_GITLAB_WINDOWS_RUNNER_TAG"]
		github: runner: "windows-latest"
	}
}

graph: {
	[#JobID]: #Job
	"source-and-governance": {stage: "verify", rank: 0, needs: []}
	"native-darwin": {stage: "verify", rank: 0, needs: []}
	"native-linux": {stage: "verify", rank: 0, needs: []}
	"native-windows": {stage: "verify", rank: 0, needs: []}
	"release-readiness": {stage: "verify", rank: 0, needs: []}
	package: {
		stage: "package"
		rank:  1
		needs: ["source-and-governance", "native-darwin", "native-linux", "native-windows", "release-readiness"]
	}
	publish: {stage: "publish", rank: 2, needs: ["package"]}
	release: {stage: "release", rank: 3, needs: ["publish"]}
}

_graphOrder: {
	for id, job in graph {
		for dependency in job.needs {
			"\(id) after \(dependency)": graph[dependency].rank < job.rank
		}
	}
}

miseImage: "ghcr.io/jdx/mise@sha256:92dbc3f2573926d8974e4641ad8449f16c323130b9f41c39aff19b7b2f500ef6"

#MiseGitLabImage: {
	name:       miseImage
	entrypoint: [""]
}

actions: {
	checkout: "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"
	mise:     "jdx/mise-action@7e36c90d9ab29c415a2384db3006f3ec8a8cc654"
	upload:   "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"
}

#SourceCheckout: {
	name: "Check out full history and tags"
	uses: actions.checkout
	with: "fetch-depth": 0
}

#ReleaseCheckout: {
	name: "Check out the exact release tag"
	uses: actions.checkout
	with: {
		ref:           "${{ inputs.tag || github.ref_name }}"
		"fetch-depth": 0
	}
}

#Toolchain: {
	name: "Install the locked toolchain"
	uses: actions.mise
	with: {
		install: true
		cache:   false
	}
}

#NativeGitHubJob: {
	_platform:         #OperatingSystem
	name:              "Native \(nativeEvidence[_platform].name) acceptance"
	"runs-on":         nativeEvidence[_platform].github.runner
	"timeout-minutes": 25
	steps: [
		#SourceCheckout,
		#Toolchain,
		{name: "Run native \(nativeEvidence[_platform].name) acceptance", run: commands.native[_platform]},
	]
}

gitlab: {
	workflow: rules: [
		{if: "$CI_COMMIT_BRANCH =~ /^release\\// && !$CI_COMMIT_TAG", when: "never"},
		{if: "$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\\//", when: "never"},
		{when: "always"},
	]
	stages: ["verify", "package", "publish", "release"]
	variables: {
		GIT_DEPTH: "0"
		GOPROXY:   "https://goproxy.cn|https://proxy.golang.org|direct"
	}
	".linux-toolchain": {
		image: #MiseGitLabImage
		"before_script": [commands.bootstrap]
	}
	"source-and-governance": {
		stage: graph["source-and-governance"].stage
		extends: [".linux-toolchain"]
		tags:  nativeEvidence.linux.gitlab.tags
		variables: AIGW_FORGE_PROVIDER: "gitlab"
		script: [
			"export AIGW_RELEASE_ALLOWED_SIGNERS_FILE=\"$AIGW_RELEASE_ALLOWED_SIGNERS\"",
			commands.source,
		]
	}
	"native-darwin": {
		stage: graph["native-darwin"].stage
		tags:  nativeEvidence.darwin.gitlab.tags
		variables: nativeToolchain
		script: [commands.install, commands.native.darwin]
	}
	"native-linux": {
		stage: graph["native-linux"].stage
		extends: [".linux-toolchain"]
		tags:  nativeEvidence.linux.gitlab.tags
		variables: nativeToolchain
		script: [commands.native.linux]
	}
	"native-windows": {
		stage: graph["native-windows"].stage
		tags:  nativeEvidence.windows.gitlab.tags
		variables: nativeToolchain
		script: [commands.install, commands.native.windows]
	}
	"release-readiness": {
		stage: graph["release-readiness"].stage
		extends: [".linux-toolchain"]
		tags:  nativeEvidence.linux.gitlab.tags
		rules: [
			{if: "$CI_COMMIT_TAG && $CI_COMMIT_TAG !~ /-(rc|beta|alpha)\\./"},
			{when: "never"},
		]
		script: ["mise exec --locked -- go run ./tools/release validate-readiness-tag"]
	}
	package: {
		stage: graph.package.stage
		tags:  nativeEvidence.darwin.gitlab.tags
		rules: [{if: "$CI_COMMIT_TAG"}, {when: "never"}]
		needs: [
			{job: "source-and-governance"},
			{job: "native-darwin"},
			{job: "native-linux"},
			{job: "native-windows"},
			{job: "release-readiness", optional: true},
		]
		script: [commands.build]
		artifacts: {
			"expire_in": "30 days"
			paths: ["dist/"]
		}
	}
	publish: {
		stage: graph.publish.stage
		tags:  nativeEvidence.darwin.gitlab.tags
		rules: [{if: "$CI_COMMIT_TAG"}, {when: "never"}]
		needs: [{job: "package", artifacts: true}]
		script: [commands.upload]
	}
	release: {
		stage: graph.release.stage
		tags:  nativeEvidence.darwin.gitlab.tags
		rules: [{if: "$CI_COMMIT_TAG"}, {when: "never"}]
		needs: [{job: "publish"}, {job: "package", artifacts: true}]
		script: [commands.publish]
	}
}

githubVerify: {
	name: "Verify"
	"on": {
		push: branches: ["main", "dev", "proposal/**"]
		"pull_request": branches: ["main", "dev"]
		"workflow_dispatch": {}
	}
	permissions: contents: "read"
	concurrency: {
		group:                "verify-${{ github.workflow }}-${{ github.ref }}"
		"cancel-in-progress": true
	}
	jobs: {
		"source-and-governance": {
			name:              "Source and governance"
			"runs-on":         nativeEvidence.linux.github.runner
			"timeout-minutes": 25
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Refresh annotated release tags", run: "mise exec --locked -- go run ./tools/ci fetch-tags"},
				{
					name: "Materialize provenance trust input"
					env: AIGW_RELEASE_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}"
					run: "mise exec --locked -- go run ./tools/ci trust-input --output \"$RUNNER_TEMP/aigw-allowed-signers\" --github-env \"$GITHUB_ENV\""
				},
				{
					name: "Verify pushed release tag provenance"
					if:   "github.ref_type == 'tag'"
					env: SELECTED_TAG: "${{ github.ref_name }}"
					run: "mise exec --locked -- go run ./tools/forge tag --provider github --tag \"$SELECTED_TAG\" --allowed-signers \"$AIGW_GITHUB_ALLOWED_SIGNERS\""
				},
				{
					name: "Run source and governance"
					env: {
						CGO_ENABLED:                       "1"
						AIGW_FORGE_PROVIDER:               "github"
						AIGW_RELEASE_AUTHOR_EMAIL:         "${{ vars.AIGW_RELEASE_AUTHOR_EMAIL }}"
						AIGW_RELEASE_ALLOWED_SIGNERS_FILE: "${{ env.AIGW_GITHUB_ALLOWED_SIGNERS }}"
						AIGW_CHANGELOG_RELEASE_TAG:        "${{ github.ref_type == 'tag' && github.ref_name || '' }}"
					}
					run: commands.source
				},
			]
		}
		"native-darwin": #NativeGitHubJob & {_platform: "darwin"}
		"native-linux": #NativeGitHubJob & {_platform: "linux"}
		"native-windows": #NativeGitHubJob & {_platform: "windows"}
	}
}

#ReleaseNativeJob: {
	_platform:         #OperatingSystem
	name:              "Native \(nativeEvidence[_platform].name) release acceptance"
	"runs-on":         nativeEvidence[_platform].github.runner
	"timeout-minutes": 25
	steps: [
		#ReleaseCheckout,
		#Toolchain,
		{name: "Run native \(nativeEvidence[_platform].name) release acceptance", run: commands.native[_platform]},
	]
}

githubRelease: {
	name: "Release"
	"on": {
		push: tags: ["v*"]
		"workflow_dispatch": inputs: tag: {
			description: "Existing v* tag to publish"
			required:    true
			type:        "string"
		}
	}
	permissions: contents: "write"
	concurrency: {
		group:                "release-${{ github.repository }}-${{ inputs.tag || github.ref_name }}"
		"cancel-in-progress": false
	}
	jobs: {
		"native-darwin": #ReleaseNativeJob & {_platform: "darwin"}
		"native-linux": #ReleaseNativeJob & {_platform: "linux"}
		"native-windows": #ReleaseNativeJob & {_platform: "windows"}
		"package-and-publish": {
			name:      "Package and publish independently"
			"runs-on": nativeEvidence.linux.github.runner
			needs: ["native-darwin", "native-linux", "native-windows"]
			"timeout-minutes": 45
			steps: [
				#ReleaseCheckout,
				#Toolchain,
				{
					name: "Materialize provenance trust input"
					env: AIGW_RELEASE_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}"
					run: "mise exec --locked -- go run ./tools/ci trust-input --output \"$RUNNER_TEMP/aigw-allowed-signers\" --github-env \"$GITHUB_ENV\""
				},
				{
					name: "Verify the signed tag and source"
					env: {
						SELECTED_TAG:                      "${{ inputs.tag || github.ref_name }}"
						AIGW_CHANGELOG_RELEASE_TAG:        "${{ inputs.tag || github.ref_name }}"
						AIGW_FORGE_PROVIDER:               "github"
						AIGW_RELEASE_AUTHOR_EMAIL:         "${{ vars.AIGW_RELEASE_AUTHOR_EMAIL }}"
						AIGW_RELEASE_ALLOWED_SIGNERS_FILE: "${{ env.AIGW_GITHUB_ALLOWED_SIGNERS }}"
					}
					run: commands.source
				},
				{
					name: "Build the complete release matrix"
					env: {
						SELECTED_TAG:                   "${{ inputs.tag || github.ref_name }}"
						AIGW_GITHUB_RELEASE_ORIGIN:     "${{ github.server_url }}"
						AIGW_GITHUB_RELEASE_REPOSITORY: "${{ github.repository }}"
					}
					run: commands.build
				},
				{
					name: "Publish or verify immutable GitHub release assets"
					env: {
						GH_TOKEN:      "${{ github.token }}"
						CI_COMMIT_TAG: "${{ inputs.tag || github.ref_name }}"
					}
					run: "mise exec --locked -- go run ./tools/release publish-github dist"
				},
			]
		}
	}
}
