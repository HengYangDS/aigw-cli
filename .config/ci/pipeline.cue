package ci

// pipeline.cue owns CI topology. Forge files are generated projections.

#OperatingSystem: "darwin" | "linux" | "windows"
#JobID:           "accepted-ref-parity" | "source-and-governance" | "native-darwin" | "native-linux" | "native-windows" | "release-readiness" | "package" | "publish" | "release"

#Job: {
	stage: "verify" | "package" | "publish" | "release"
	rank:  int & >=0
	needs: [...#JobID]
}

commands: {
	install:         "mise install --locked"
	installNpmTools: "npm ci --ignore-scripts"
	sourceMirror: #"""
		lock_sha=$(sha256sum mise.lock | awk '{print $1}')
		authenticated_api="$CI_SERVER_PROTOCOL://gitlab-ci-token:$CI_JOB_TOKEN@$CI_SERVER_HOST:$CI_SERVER_PORT/api/v4"
		mirror="$authenticated_api/projects/$CI_PROJECT_ID/packages/generic/ci-source-tools/$lock_sha/\$4"
		export MISE_URL_REPLACEMENTS="{\"regex:^https://github\\\\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/(.+)\$\":\"$mirror\"}"
		"""#
	source: "mise exec --locked -- go run ./tools/ci source"
	native: {
		for platform in ["darwin", "linux", "windows"] {
			(platform): "mise exec --locked -- go run ./tools/ci native --platform \(platform)"
		}
	}
	systemKeyring: linux: {
		prepare: "apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq dbus-x11 gnome-keyring"
		run:     "dbus-run-session -- sh -c 'printf \\\"\\\\n\\\" | gnome-keyring-daemon --unlock >/dev/null; export AIGW_VERIFY_SYSTEM_KEYRING=1; \(native.linux)'"
	}
	build:   "mise exec --locked -- go run ./tools/release build-ci build/release dist"
	upload:  "mise exec --locked -- go run ./tools/release upload-gitlab dist"
	publish: "mise exec --locked -- go run ./tools/release publish-gitlab dist"
	acceptedRefParity: {
		gitlab: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.acceptedBranch)=$CI_COMMIT_SHA\" --expect \"\(lifecycle.reviewBranch)=$CI_COMMIT_SHA\""
		github: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.acceptedBranch)=${{ github.sha }}\" --expect \"\(lifecycle.reviewBranch)=${{ github.sha }}\""
	}
}

nativeToolchain: MISE_ENABLE_TOOLS:           "go,cue"
sourceToolchain: MISE_ENABLE_TOOLS:           "go,node,cue,github:editorconfig-checker/editorconfig-checker,github:gitleaks/gitleaks,github:rhysd/actionlint,github:lycheeverse/lychee"
releaseReadinessToolchain: MISE_ENABLE_TOOLS: "go"

lifecycle: {
	acceptedBranch: "main"
	reviewBranch:   "dev"
	checkoutRemote: "origin"
}

// Product evidence and Forge execution capacity are separate facts. A Forge
// projects only the native jobs it can execute; product-level evidence remains
// complete across the independent publication planes.
productEvidence: native: ["darwin", "linux", "windows"]

forgeCapabilities: {
	gitlab: {
		darwin:  true
		linux:   true
		windows: false
	}
	github: {
		for platform in productEvidence.native {
			(platform): true
		}
	}
}

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
		github: runner: "windows-latest"
	}
}

graph: {
	[#JobID]: #Job
	"accepted-ref-parity": {stage: "verify", rank: 0, needs: []}
	"source-and-governance": {stage: "verify", rank: 0, needs: []}
	"native-darwin": {stage: "verify", rank: 0, needs: []}
	"native-linux": {stage: "verify", rank: 0, needs: []}
	"native-windows": {stage: "verify", rank: 0, needs: []}
	"release-readiness": {stage: "verify", rank: 0, needs: []}
	package: {
		stage: "package"
		rank:  1
		needs: ["source-and-governance", "native-darwin", "native-linux", "release-readiness"]
	}
	publish: {stage: "publish", rank: 2, needs: ["package"]}
	release: {stage: "release", rank: 3, needs: ["publish"]}
}

gitlabFullVerificationRules: [
	{if: "$CI_COMMIT_TAG"},
	{if: "$CI_PIPELINE_SOURCE == \"merge_request_event\" && $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == \"\(lifecycle.reviewBranch)\""},
	{if: "$CI_PIPELINE_SOURCE == \"push\" && $CI_COMMIT_BRANCH == \"\(lifecycle.acceptedBranch)\""},
	{if: "$CI_PIPELINE_SOURCE == \"web\" || $CI_PIPELINE_SOURCE == \"api\""},
	{when: "never"},
]

githubFullVerificationCondition: "github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || github.ref_name == '\(lifecycle.acceptedBranch)'"

_graphOrder: {
	for id, job in graph {
		for dependency in job.needs {
			"\(id) after \(dependency)": graph[dependency].rank < job.rank
		}
	}
}

miseImage: "ghcr.io/jdx/mise@sha256:df5adf538c92d86f9807e738c7c7a9b32a1ae06edf441ede141333e3fe6a2cd9"

#MiseGitLabImage: {
	name: miseImage
	entrypoint: [""]
}

actions: {
	checkout: "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"
	mise:     "jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c"
	upload:   "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"
}

#SourceCheckout: {
	name: "Check out the exact product commit"
	uses: actions.checkout
	with: {
		ref:           "${{ github.event.pull_request.head.sha || github.sha }}"
		"fetch-depth": 0
	}
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
	if:                githubFullVerificationCondition
	if _platform == "linux" {
		steps: [
			#SourceCheckout,
			#Toolchain,
			{name: "Install Secret Service", run: "sudo -- sh -c '\(commands.systemKeyring.linux.prepare)'"},
			{name: "Run native Linux acceptance", run: commands.systemKeyring.linux.run},
		]
	}
	if _platform != "linux" {
		steps: [
			#SourceCheckout,
			#Toolchain,
			{
				name: "Run native \(nativeEvidence[_platform].name) acceptance"
				env: {
					AIGW_VERIFY_SYSTEM_KEYRING: "1"
					if _platform == "darwin" {
						AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE: "ephemeral-host"
					}
				}
				run: commands.native[_platform]
			},
		]
	}
}

#NativeGitLabJob: {
	_platform: #OperatingSystem
	stage:     graph["native-\(_platform)"].stage
	tags:      nativeEvidence[_platform].gitlab.tags
	variables: nativeToolchain
	rules:     gitlabFullVerificationRules
	if _platform == "linux" {
		extends: [".linux-toolchain"]
		script: [commands.systemKeyring.linux.prepare, commands.systemKeyring.linux.run]
	}
	if _platform != "linux" {
		script: [commands.install, commands.native[_platform]]
	}
}

gitlab: {
	variables: {
		GIT_DEPTH:       "0"
		GOPROXY:         "https://goproxy.cn|https://proxy.golang.org|direct"
		MISE_CONFIG_DIR: "$CI_PROJECT_DIR/.config/ci"
	}
	workflow: rules: [
		{if: "$CI_COMMIT_TAG"},
		{if: "$CI_PIPELINE_SOURCE == \"merge_request_event\" && $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == \"\(lifecycle.reviewBranch)\""},
		{if: "$CI_PIPELINE_SOURCE == \"push\" && $CI_COMMIT_BRANCH == \"\(lifecycle.acceptedBranch)\""},
		{if: "$CI_PIPELINE_SOURCE == \"web\" || $CI_PIPELINE_SOURCE == \"api\""},
		{when: "never"},
	]
	stages: ["verify", "package", "publish", "release"]
	".linux-toolchain": {
		image: #MiseGitLabImage
	}
	".source-toolchain": {
		image: #MiseGitLabImage
		"before_script": [commands.sourceMirror, commands.install]
	}
	"source-and-governance": {
		stage: graph["source-and-governance"].stage
		extends: [".source-toolchain"]
		tags:      nativeEvidence.linux.gitlab.tags
		variables: sourceToolchain
		rules:     gitlabFullVerificationRules
		script: [
			commands.installNpmTools,
			"export AIGW_RELEASE_ALLOWED_SIGNERS_FILE=\"$AIGW_RELEASE_ALLOWED_SIGNERS\"",
			commands.source,
		]
	}
	"accepted-ref-parity": {
		stage: graph["accepted-ref-parity"].stage
		extends: [".linux-toolchain"]
		tags:      nativeEvidence.linux.gitlab.tags
		variables: releaseReadinessToolchain
		rules: [
			{if: "$CI_PIPELINE_SOURCE == \"push\" && $CI_COMMIT_BRANCH == \"\(lifecycle.acceptedBranch)\""},
			{when: "never"},
		]
		script: [commands.install, commands.acceptedRefParity.gitlab]
	}
	"native-darwin": #NativeGitLabJob & {_platform: "darwin"}
	"native-linux": #NativeGitLabJob & {_platform: "linux"}
	if forgeCapabilities.gitlab.windows {
		"native-windows": #NativeGitLabJob & {_platform: "windows"}
	}
	"release-readiness": {
		stage: graph["release-readiness"].stage
		extends: [".linux-toolchain"]
		tags:      nativeEvidence.linux.gitlab.tags
		variables: releaseReadinessToolchain
		rules: [
			{if: "$CI_COMMIT_TAG && $CI_COMMIT_TAG !~ /-(rc|beta|alpha)\\./"},
			{when: "never"},
		]
		script: ["mise exec --locked -- go run ./tools/release validate-readiness-tag"]
	}
	package: {
		stage: graph.package.stage
		tags:  nativeEvidence.darwin.gitlab.tags
		rules: [
			{if: "$CI_COMMIT_TAG"},
			{when: "never"},
		]
		needs: [
			{job: "source-and-governance"},
			{job: "native-darwin"},
			{job: "native-linux"},
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
		rules: [
			{if: "$CI_COMMIT_TAG"},
			{when: "never"},
		]
		needs: [{job: "package", artifacts: true}]
		script: [commands.upload]
	}
	release: {
		stage: graph.release.stage
		tags:  nativeEvidence.darwin.gitlab.tags
		rules: [
			{if: "$CI_COMMIT_TAG"},
			{when: "never"},
		]
		needs: [{job: "publish"}, {job: "package", artifacts: true}]
		script: [commands.publish]
	}
}

githubVerify: {
	name: "Verify"
	env: {
		GIT_CONFIG_COUNT:   "1"
		GIT_CONFIG_KEY_0:   "init.defaultBranch"
		GIT_CONFIG_VALUE_0: "main"
		MISE_CONFIG_DIR:    "${{ github.workspace }}/.config/ci"
	}
	"on": {
		push: branches: [lifecycle.acceptedBranch]
		"pull_request": branches: [lifecycle.reviewBranch]
		"workflow_dispatch": {}
	}
	permissions: contents: "read"
	concurrency: {
		group:                "verify-${{ github.workflow }}-${{ github.ref }}"
		"cancel-in-progress": true
	}
	jobs: {
		"accepted-ref-parity": {
			name:              "Accepted ref parity"
			"runs-on":         nativeEvidence.linux.github.runner
			"timeout-minutes": 5
			if:                "github.event_name == 'push' && github.ref_name == '\(lifecycle.acceptedBranch)'"
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Verify main and dev name the same product object", run: commands.acceptedRefParity.github},
			]
		}
		"source-and-governance": {
			name:              "Source and governance"
			"runs-on":         nativeEvidence.linux.github.runner
			"timeout-minutes": 25
			if:                githubFullVerificationCondition
			env:               sourceToolchain
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Install locked npm tools", run: commands.installNpmTools},
				{
					name: "Materialize provenance trust input"
					env: AIGW_RELEASE_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}"
					run: "mise exec --locked -- go run ./tools/ci trust-input --output \"$RUNNER_TEMP/aigw-allowed-signers\" --github-env \"$GITHUB_ENV\""
				},
				{
					name: "Verify pushed release tag provenance"
					if:   "github.ref_type == 'tag'"
					env: SELECTED_TAG: "${{ github.ref_name }}"
					run: "mise exec --locked -- go run ./tools/forge tag --tag \"$SELECTED_TAG\" --allowed-signers \"$AIGW_RELEASE_ALLOWED_SIGNERS_FILE\""
				},
				{
					name: "Run source and governance"
					env: {
						CGO_ENABLED:                       "1"
						AIGW_RELEASE_AUTHOR_EMAIL:         "${{ vars.AIGW_RELEASE_AUTHOR_EMAIL }}"
						AIGW_RELEASE_ALLOWED_SIGNERS_FILE: "${{ env.AIGW_RELEASE_ALLOWED_SIGNERS_FILE }}"
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
	env: {
		GIT_CONFIG_COUNT:   "1"
		GIT_CONFIG_KEY_0:   "init.defaultBranch"
		GIT_CONFIG_VALUE_0: "main"
		MISE_CONFIG_DIR:    "${{ github.workspace }}/.config/ci"
	}
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
				{name: "Install locked npm tools", run: commands.installNpmTools},
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
						AIGW_RELEASE_AUTHOR_EMAIL:         "${{ vars.AIGW_RELEASE_AUTHOR_EMAIL }}"
						AIGW_RELEASE_ALLOWED_SIGNERS_FILE: "${{ env.AIGW_RELEASE_ALLOWED_SIGNERS_FILE }}"
					}
					run: commands.source
				},
				{
					name: "Build the complete release matrix"
					env: {
						CI_COMMIT_TAG:                  "${{ inputs.tag || github.ref_name }}"
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
