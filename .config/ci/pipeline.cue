package ci

import "strings"

// pipeline.cue owns CI topology. Forge files are generated projections.

#OperatingSystem: "darwin" | "linux" | "windows"
#JobID:           "accepted-ref-parity" | "quality" | "native-darwin" | "native-linux" | "native-windows" | "release-readiness" | "release-assets"
#Claim:           "accepted-ref-parity" | "source-quality" | "go-source-compatibility" | "native-product-journey" | "lifecycle-acceptance" | "release-metadata" | "artifact-verification"

#Job: {
	stage: "verify" | "release"
	rank:  int & >=0
	needs: [...#JobID]
	claims: [#Claim, ...#Claim]
}

// Go's module and checksum hosts reset HTTP/2 streams during cold installs.
// Keep this transport choice in the installer process, not product execution.
installationEnvironment: GODEBUG: "http2client=0"

commands: {
	install:      "env GODEBUG=\(installationEnvironment.GODEBUG) mise install --locked"
	bootstrap:    "mise run bootstrap"
	resolveLocks: "mise run dependencies:resolve"
	quality:      "mise exec --locked -- go run ./tools/ci quality"
	source:       "mise exec --locked -- go run ./tools/ci source"
	native: {
		for platform in ["darwin", "linux", "windows"] {
			(platform): "mise exec --locked -- go run ./tools/ci native --platform \(platform)"
		}
	}
	readiness: "mise exec --locked -- go run ./tools/release validate-readiness-tag"
	artifacts: "mise exec --locked -- go run ./tools/release verify-artifacts dist"
	acceptedRefParity: {
		gitlab: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.releaseBranch)=$CI_COMMIT_SHA\" --expect \"\(lifecycle.acceptedBranch)=$CI_COMMIT_SHA\""
		github: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.releaseBranch)=${{ github.sha }}\" --expect \"\(lifecycle.acceptedBranch)=${{ github.sha }}\""
	}
}

goToolchain: MISE_ENABLE_TOOLS:      "go"
nativeToolchain: MISE_ENABLE_TOOLS:  "\(qualityToolchain.MISE_ENABLE_TOOLS),gh,glab,github:anchore/syft"
qualityToolchain: MISE_ENABLE_TOOLS: "go,node,cue,github:boyter/scc,github:editorconfig-checker/editorconfig-checker,github:gitleaks/gitleaks,github:golangci/golangci-lint,github:goreleaser/goreleaser,go:github.com/google/osv-scanner/v2/cmd/osv-scanner,github:lycheeverse/lychee,github:rhysd/actionlint,taplo"

// Git role names belong to the adopter workspace; CUE consumes its native TOML.
branch_roles: {accepted_branch: string, release_branch: string}

lifecycle: {
	releaseBranch:  branch_roles.release_branch
	acceptedBranch: branch_roles.accepted_branch
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

// AIGW is an application built with one locked Go toolchain, not a Go library
// supporting a range of compilers. Native jobs therefore form its meaningful
// Go compatibility matrix without adding a duplicate same-version test job.
goSourceMatrix: {
	versionSource: "go.mod"
	toolchainLock: "mise.lock"
	platforms: {
		for platform in productEvidence.native {
			(platform): {
				job:     "native-\(platform)"
				command: commands.native[platform]
			}
		}
	}
}

// Each event performs its own verification. Published artifacts are inputs,
// not inherited proof; selected source, trust and complete bytes are rechecked.
evidenceReuse: {
	crossRun: false
	invalidatedBy: [
		"source",
		"platform",
		"environment",
		"dependency-locks",
		"toolchain",
		"release-identity",
		"claimed-fact",
	]
	samePipeline: []
}

graph: {
	[#JobID]: #Job
	"accepted-ref-parity": {stage: "verify", rank: 0, needs: [], claims: ["accepted-ref-parity"]}
	quality: {stage: "verify", rank: 0, needs: [], claims: ["source-quality"]}
	"native-darwin": {stage: "verify", rank: 0, needs: [], claims: ["go-source-compatibility", "native-product-journey", "lifecycle-acceptance"]}
	"native-linux": {stage: "verify", rank: 0, needs: [], claims: ["go-source-compatibility", "native-product-journey", "lifecycle-acceptance"]}
	"native-windows": {stage: "verify", rank: 0, needs: [], claims: ["go-source-compatibility", "native-product-journey", "lifecycle-acceptance"]}
	"release-readiness": {stage: "verify", rank: 0, needs: [], claims: ["release-metadata"]}
	"release-assets": {stage: "release", rank: 1, needs: ["quality", "native-darwin", "native-linux", "release-readiness"], claims: ["artifact-verification"]}
}

gitlabVerificationCondition: {
	tag:    "$CI_COMMIT_TAG"
	review: "$CI_PIPELINE_SOURCE == \"merge_request_event\" && ($CI_MERGE_REQUEST_TARGET_BRANCH_NAME == \"\(lifecycle.acceptedBranch)\" || $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == \"\(lifecycle.releaseBranch)\")"
	push:   "$CI_PIPELINE_SOURCE == \"push\" && ($CI_COMMIT_BRANCH == \"\(lifecycle.acceptedBranch)\" || $CI_COMMIT_BRANCH == \"\(lifecycle.releaseBranch)\")"
	manual: "$CI_PIPELINE_SOURCE == \"web\" || $CI_PIPELINE_SOURCE == \"api\""
}

gitlabFullVerificationRules: [
	{if: gitlabVerificationCondition.tag},
	{if: gitlabVerificationCondition.review},
	{if: gitlabVerificationCondition.push},
	{if: gitlabVerificationCondition.manual},
	{when: "never"},
]

githubFullVerificationCondition: "github.ref_type == 'tag' || github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || github.ref_name == '\(lifecycle.acceptedBranch)' || github.ref_name == '\(lifecycle.releaseBranch)'"

githubCommitBase: "${{ github.event.pull_request.base.sha || (github.ref_type == 'tag' && format('{0}^', github.sha)) || github.event.before || inputs.commit_base }}"

_graphOrder: {
	for id, job in graph {
		for dependency in job.needs {
			"\(id) after \(dependency)": graph[dependency].rank < job.rank
		}
	}
}

miseImage: "ghcr.io/jdx/mise:2026.9.7@sha256:f7e1136dac52ed4e6fe7d54824dbb555a2f12267f7522eaaeecf69eded5d12e1"

#MiseGitLabImage: {
	name: miseImage
	entrypoint: [""]
}

actions: {
	checkout: "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"        // v7.0.1
	mise:     "jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c"         // v4.3.0
	upload:   "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a" // v7.0.1
}

#ReleaseTrustFiles: {
	AIGW_RELEASE_ALLOWED_SIGNERS_FILE:          "$AIGW_RELEASE_ALLOWED_SIGNERS"
	AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE: "$AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS"
	SSH_ASKPASS_REQUIRE:                        "never"
	GLAB_ENABLE_CI_AUTOLOGIN:                   "true"
	MISE_ENABLE_TOOLS:                          "go,glab"
}

#SourceCheckout: {
	name: "Check out the exact product commit"
	uses: actions.checkout
	with: {
		ref:           "${{ github.event.pull_request.head.sha || github.sha }}"
		"fetch-depth": 0
	}
}

#Toolchain: {
	name: "Install the locked toolchain"
	uses: actions.mise
	env:  installationEnvironment
	with: {
		version:          strings.Split(strings.Split(miseImage, ":")[1], "@")[0]
		install:          true
		install_args:     "--locked"
		cache:            true
		cache_key_prefix: "mise-${{ github.job }}"
	}
}

#NativeGitHubJob: {
	_platform: #OperatingSystem
	_credentialEnvironment: {
		if _platform != "linux" {
			AIGW_VERIFY_SYSTEM_KEYRING: "1"
			if _platform == "darwin" {
				AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE: "ephemeral-host"
			}
		}
	}
	name:              "Native \(nativeEvidence[_platform].name) acceptance"
	"runs-on":         nativeEvidence[_platform].github.runner
	"timeout-minutes": 25
	if:                githubFullVerificationCondition
	env:               nativeToolchain
	steps: [
		#SourceCheckout,
		#Toolchain,
		{name: "Prepare locked dependencies", run: commands.bootstrap},
		if _platform == "linux" {
			name: "Prepare native memory measurement"
			if:   "github.event_name == 'workflow_dispatch' && inputs.performance"
			run:  "sudo apt-get update -qq && sudo DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y time"
		},
		{
			name: "Verify native lock resolution"
			if:   "github.event_name == 'workflow_dispatch' && inputs.refresh_locks"
			env: MISE_GITHUB_TOKEN: "${{ github.token }}"
			run: commands.resolveLocks
		},
		{
			name: "Retain native lock observation"
			if:   "always() && github.event_name == 'workflow_dispatch' && inputs.refresh_locks"
			uses: actions.upload
			with: {
				name: "mise-lock-\(_platform)"
				path: "mise.lock"
			}
		},
		for full in [false, true] {
			if !full {
				name: "Run native \(nativeEvidence[_platform].name) acceptance"
				if:   "github.event_name != 'workflow_dispatch' || !inputs.full_quality"
				run:  commands.native[_platform]
			}
			if full {
				name: "Qualify all repository tools on \(nativeEvidence[_platform].name)"
				if:   "github.event_name == 'workflow_dispatch' && inputs.full_quality"
				run:  "\(commands.native[_platform]) --full-quality"
			}
			if full || _platform != "linux" {
				env: _credentialEnvironment & {
					if full {CGO_ENABLED: "1"}
				}
			}
		},
		{
			name:  "Run historical release acceptance"
			if:    "github.event_name == 'workflow_dispatch' && (inputs.baseline_tag != '' || inputs.candidate_tag != '' || inputs.windows_clients || inputs.performance)"
			shell: "pwsh"
			env: _credentialEnvironment & {
				GH_TOKEN:                              "${{ github.token }}"
				AIGW_BASELINE_TAG:                     "${{ inputs.baseline_tag }}"
				AIGW_CANDIDATE_TAG:                    "${{ inputs.candidate_tag }}"
				AIGW_RELEASE_ALLOWED_SIGNERS:          "${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}"
				AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS }}"
				AIGW_RELEASE_ARTIFACT_SIGNER:          "${{ vars.AIGW_RELEASE_ARTIFACT_SIGNER }}"
				AIGW_QUALIFY_WINDOWS_CLIENTS:          "${{ inputs.windows_clients }}"
				AIGW_MEASURE_PERFORMANCE:              "${{ inputs.performance }}"
			}
			run: #"""
				$ErrorActionPreference = 'Stop'
				$PSNativeCommandUseErrorActionPreference = $true
				if ([string]::IsNullOrWhiteSpace($env:AIGW_BASELINE_TAG)) { throw 'Client qualification requires baseline_tag' }
				if ($env:AIGW_MEASURE_PERFORMANCE -eq 'true' -and [string]::IsNullOrWhiteSpace($env:AIGW_CANDIDATE_TAG)) { throw 'Performance qualification requires candidate_tag' }
				$scope = Join-Path $env:RUNNER_TEMP ([guid]::NewGuid().ToString())
				New-Item -ItemType Directory -Path $scope | Out-Null
				try {
				  $platform = mise exec --locked -- go env GOOS
				  $architecture = mise exec --locked -- go env GOARCH
				  $extension = if ($platform -eq 'windows') { 'zip' } else { 'tar.gz' }
				  $pattern = "*_${platform}_${architecture}.${extension}"
				  gh release download $env:AIGW_BASELINE_TAG --repo $env:GITHUB_REPOSITORY --pattern $pattern --pattern checksums.txt --dir $scope
				  $archives = @(Get-ChildItem -Path $scope -Filter $pattern -File)
				  if ($archives.Count -ne 1) { throw 'Expected exactly one native release archive' }
				  $archive = $archives[0]
				  $entries = @(Get-Content (Join-Path $scope 'checksums.txt') | ForEach-Object {
				    $entry = $_ -split '\s+', 2
				    if ($entry.Count -eq 2 -and $entry[1].TrimStart('*') -eq $archive.Name) { $entry[0] }
				  })
				  $actual = (Get-FileHash $archive.FullName -Algorithm SHA256).Hash
				  if ($entries.Count -ne 1 -or $entries[0] -notmatch '^[0-9a-fA-F]{64}$' -or $entries[0] -ne $actual) {
				    throw 'Historical release archive checksum mismatch'
				  }
				  tar -xf $archive.FullName -C $scope
				  $program = if ($platform -eq 'windows') { 'aigw.exe' } else { 'aigw' }
				  $executables = @(Get-ChildItem -Path $scope -Recurse -Filter $program -File)
				  if ($executables.Count -ne 1) { throw 'Expected exactly one historical executable' }
				  $env:AIGW_ACCEPTANCE_BASELINE = $executables[0].FullName
				  Write-Output "Historical release $env:AIGW_BASELINE_TAG archive SHA256=$actual"
				  $acceptance = @('accept-native')
				  if (-not [string]::IsNullOrWhiteSpace($env:AIGW_CANDIDATE_TAG)) {
				    $candidate = Join-Path $scope 'candidate'
				    New-Item -ItemType Directory -Path $candidate | Out-Null
				    mise exec --locked -- gh release download $env:AIGW_CANDIDATE_TAG --repo $env:GITHUB_REPOSITORY --dir $candidate
				    $env:CI_COMMIT_TAG = $env:AIGW_CANDIDATE_TAG
				    $env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE = Join-Path $scope 'source-signers'
				    $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE = Join-Path $scope 'artifact-signers'
				    [IO.File]::WriteAllText($env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE, $env:AIGW_RELEASE_ALLOWED_SIGNERS)
				    [IO.File]::WriteAllText($env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE, $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS)
				    $acceptance += @('--artifacts', $candidate)
				  }
				  if ($platform -eq 'windows' -and $env:AIGW_QUALIFY_WINDOWS_CLIENTS -eq 'true') {
				    $clients = Join-Path $scope 'clients'
				    New-Item -ItemType Directory -Path $clients | Out-Null
				    Set-Content -LiteralPath (Join-Path $clients 'package.json') -Value '{"private":true}'
				    mise exec --locked -- npm install --prefix $clients --ignore-scripts --save-exact --no-audit --no-fund '@openai/codex@0.154.0' '@anthropic-ai/claude-code-win32-x64@2.1.269'
				    mise exec --locked -- npm audit signatures --prefix $clients
				    $codexRoot = Join-Path $clients 'node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc'
				    $env:AIGW_ACCEPTANCE_CODEX = Join-Path $codexRoot 'bin/codex.exe'
				    $env:AIGW_ACCEPTANCE_CLAUDE = Join-Path $clients 'node_modules/@anthropic-ai/claude-code-win32-x64/claude.exe'
				    $gitBin = Split-Path (Get-Command git.exe).Source
				    $bash = Join-Path (Split-Path $gitBin) 'bin/bash.exe'
				    if (-not (Test-Path -LiteralPath $bash -PathType Leaf)) { throw 'Claude requires a native Git Bash installation' }
				    $env:CLAUDE_CODE_GIT_BASH_PATH = $bash
				    $env:AIGW_ACCEPTANCE_CLIENT_PATH = @((Join-Path $codexRoot 'codex-path'), $gitBin, (Split-Path $bash), (Join-Path $env:SystemRoot 'System32')) -join ';'
				    foreach ($client in @($env:AIGW_ACCEPTANCE_CODEX, $env:AIGW_ACCEPTANCE_CLAUDE)) {
				      if (-not (Test-Path -LiteralPath $client -PathType Leaf)) { throw "Missing native client: $client" }
				      Get-FileHash -LiteralPath $client -Algorithm SHA256
				    }
				    $acceptance += '--clients'
				  }
				  Remove-Item Env:GH_TOKEN -ErrorAction SilentlyContinue
				  if ($env:AIGW_MEASURE_PERFORMANCE -eq 'true') {
				    $env:MISE_ENABLE_TOOLS += ',github:sharkdp/hyperfine'
				    $output = Join-Path $env:GITHUB_WORKSPACE 'build/performance'
				    $performance = $acceptance[1..($acceptance.Count - 1)] + @('--performance', $output)
				    mise run performance @performance
				  } else {
				    mise exec --locked -- go run ./tools/release @acceptance
				  }
				} finally {
				  foreach ($name in @('AIGW_ACCEPTANCE_BASELINE', 'AIGW_ACCEPTANCE_CODEX', 'AIGW_ACCEPTANCE_CLAUDE', 'AIGW_ACCEPTANCE_CLIENT_PATH', 'CLAUDE_CODE_GIT_BASH_PATH')) {
				    Remove-Item "Env:$name" -ErrorAction SilentlyContinue
				  }
				  Remove-Item -LiteralPath $scope -Recurse -Force
				}
				"""#
		},
		{
			name: "Retain native performance samples"
			if:   "always() && github.event_name == 'workflow_dispatch' && inputs.performance"
			uses: actions.upload
			with: {
				name:                "performance-\(_platform)"
				path:                "build/performance"
				"if-no-files-found": "error"
			}
		},
	]
}

#NativeGitLabJob: {
	_platform:     #OperatingSystem
	_install:      string
	_refreshLocks: string
	_native:       string
	if _platform == "windows" {
		_install:      "cmd /c \"set GODEBUG=\(installationEnvironment.GODEBUG)&&mise install --locked\""
		_refreshLocks: "if ($env:AIGW_REFRESH_LOCKS -eq 'true') { \(commands.resolveLocks) }"
		_native:       "\(commands.native[_platform]) --full-quality=\"$($env:AIGW_FULL_NATIVE_QUALITY -eq 'true')\""
	}
	if _platform != "windows" {
		_install:      commands.install
		_refreshLocks: "if [ \"${AIGW_REFRESH_LOCKS:-false}\" = true ]; then \(commands.resolveLocks); fi"
		_native:       "\(commands.native[_platform]) --full-quality=\"${AIGW_FULL_NATIVE_QUALITY:-false}\""
	}
	stage:     graph["native-\(_platform)"].stage
	tags:      nativeEvidence[_platform].gitlab.tags
	variables: nativeToolchain
	rules:     gitlabFullVerificationRules
	artifacts: {
		when: "always"
		paths: ["mise.lock"]
	}
	if _platform == "linux" {
		extends: [".linux-toolchain"]
		script: [commands.bootstrap, _refreshLocks, _native]
	}
	if _platform != "linux" {
		script: [_install, commands.bootstrap, _refreshLocks, _native]
	}
}

gitlab: {
	variables: {
		GIT_DEPTH: "0"
		GOPROXY:   "https://goproxy.cn|https://proxy.golang.org|direct"
	}
	workflow: rules: gitlabFullVerificationRules
	stages: ["verify", "release"]
	".linux-toolchain": {
		_dataDirectory: "build/runtime/tool-cache/.mise"
		_cacheDirectories: ["installs", "cache"]
		image: #MiseGitLabImage
		variables: {
			MISE_DATA_DIR:  "$CI_PROJECT_DIR/\(_dataDirectory)"
			MISE_CACHE_DIR: "$CI_PROJECT_DIR/\(_dataDirectory)/cache"
		}
		cache: {
			key: {
				files: ["mise.toml", "mise.lock"]
				prefix: "mise-linux-\(strings.Split(miseImage, "@sha256:")[1])-\(strings.Join(strings.Split(_dataDirectory, "/"), "-"))-\(strings.Join(_cacheDirectories, "-"))-$CI_RUNNER_ID-$CI_JOB_NAME-$CI_COMMIT_REF_SLUG"
			}
			paths: [for directory in _cacheDirectories {"\(_dataDirectory)/\(directory)/"}]
			policy: "pull-push"
			when:   "always"
		}
		"before_script": [commands.install]
	}
	quality: {
		stage: graph.quality.stage
		extends: [".linux-toolchain"]
		tags:      nativeEvidence.linux.gitlab.tags
		variables: qualityToolchain
		rules: [
			{if: gitlabVerificationCondition.tag, variables: AIGW_COMMIT_BASE: "$CI_COMMIT_SHA^"},
			{
				if: gitlabVerificationCondition.review
				variables: AIGW_COMMIT_BASE: "$CI_MERGE_REQUEST_DIFF_BASE_SHA"
			},
			{
				if: gitlabVerificationCondition.push
				variables: AIGW_COMMIT_BASE: "$CI_COMMIT_BEFORE_SHA"
			},
			{if: gitlabVerificationCondition.manual},
			{when: "never"},
		]
		script: [
			commands.bootstrap,
			"export AIGW_RELEASE_ALLOWED_SIGNERS_FILE=\"$AIGW_RELEASE_ALLOWED_SIGNERS\"",
			commands.quality,
		]
	}
	"accepted-ref-parity": {
		stage: graph["accepted-ref-parity"].stage
		extends: [".linux-toolchain"]
		tags:      nativeEvidence.linux.gitlab.tags
		variables: goToolchain
		rules: [
			{if: "$CI_PIPELINE_SOURCE == \"push\" && $CI_COMMIT_BRANCH == \"\(lifecycle.releaseBranch)\""},
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
		variables: goToolchain
		rules: [
			{if: "$CI_COMMIT_TAG"},
			{when: "never"},
		]
		script: [commands.readiness]
	}
	"release-assets": {
		stage: graph["release-assets"].stage
		extends: [".linux-toolchain"]
		tags:      nativeEvidence.linux.gitlab.tags
		variables: #ReleaseTrustFiles
		rules: [
			{if: "$CI_COMMIT_TAG && ($CI_PIPELINE_SOURCE == \"api\" || $CI_PIPELINE_SOURCE == \"web\")"},
			{when: "never"},
		]
		needs: [for dependency in graph["release-assets"].needs {{job: dependency}}]
		script: [
			#"mkdir dist"#,
			#"mise exec --locked -- glab release download "$CI_COMMIT_TAG" --repo "$CI_PROJECT_URL" --asset-name 'aigw_*' --asset-name 'checksums.txt*' --dir dist"#,
			commands.artifacts,
		]
	}

}

githubVerify: {
	name: "Verify"
	env: {
		GIT_CONFIG_COUNT:   "1"
		GIT_CONFIG_KEY_0:   "init.defaultBranch"
		GIT_CONFIG_VALUE_0: "main"
	}
	"on": {
		push: {branches: [lifecycle.acceptedBranch, lifecycle.releaseBranch], tags: ["v*"]}
		"pull_request": branches: [lifecycle.acceptedBranch, lifecycle.releaseBranch]
		"workflow_dispatch": inputs: {
			full_quality: {
				description: "Qualify all repository quality tools on each native platform"
				required:    false
				type:        "boolean"
				default:     false
			}
			windows_clients: {
				description: "With baseline_tag, qualify real Windows clients through the existing release lifecycle"
				required:    false
				type:        "boolean"
				default:     false
			}
			performance: {
				description: "Measure published candidate_tag against baseline_tag with retained native samples"
				required:    false
				type:        "boolean"
				default:     false
			}
			refresh_locks: {
				description: "Verify two native mise lock refreshes without changing committed inputs"
				required:    false
				type:        "boolean"
				default:     false
			}
			commit_base: {
				description: "Exclusive commit base for manual verification"
				required:    true
				type:        "string"
			}
			baseline_tag: {
				description: "Optional historical release tag for packaged upgrade and rollback acceptance"
				required:    false
				type:        "string"
			}
			candidate_tag: {
				description: "With baseline_tag, consume this published candidate without rebuilding"
				required:    false
				type:        "string"
			}
		}
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
			if:                "github.event_name == 'push' && github.ref_name == '\(lifecycle.releaseBranch)'"
			env:               goToolchain
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Verify main and dev name the same product object", run: commands.acceptedRefParity.github},
			]
		}
		quality: {
			name:              "Quality and governance"
			"runs-on":         nativeEvidence.linux.github.runner
			"timeout-minutes": 25
			if:                githubFullVerificationCondition
			env:               qualityToolchain
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Prepare locked dependencies", run: commands.bootstrap},
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
					name: "Run quality and governance"
					env: {
						CGO_ENABLED:                       "1"
						AIGW_COMMIT_BASE:                  githubCommitBase
						AIGW_RELEASE_AUTHOR_EMAIL:         "${{ vars.AIGW_RELEASE_AUTHOR_EMAIL }}"
						AIGW_RELEASE_ALLOWED_SIGNERS_FILE: "${{ env.AIGW_RELEASE_ALLOWED_SIGNERS_FILE }}"
						AIGW_CHANGELOG_RELEASE_TAG:        "${{ github.ref_type == 'tag' && github.ref_name || '' }}"
					}
					run: commands.quality
				},
			]
		}
		"native-darwin": #NativeGitHubJob & {_platform: "darwin"}
		"native-linux": #NativeGitHubJob & {_platform: "linux"}
		"native-windows": #NativeGitHubJob & {_platform: "windows"}
	}
}

githubRelease: {
	name: "Release"
	env: {
		GIT_CONFIG_COUNT:   "1"
		GIT_CONFIG_KEY_0:   "init.defaultBranch"
		GIT_CONFIG_VALUE_0: "main"
	}
	"on": {
		"workflow_dispatch": inputs: {
			tag: {
				description: "Existing published v* release to verify"
				required:    true
				type:        "string"
			}
			runner: {
				description: "Native execution environment"
				type:        "choice"
				default:     nativeEvidence.linux.github.runner
				options: [
					nativeEvidence.linux.github.runner,
					"ubuntu-24.04-arm",
					nativeEvidence.darwin.github.runner,
					"macos-15-intel",
					nativeEvidence.windows.github.runner,
					"windows-11-arm",
				]
			}
			native_lifecycle: {
				description: "Run the existing native lifecycle against the published bytes"
				type:        "boolean"
				default:     false
			}
		}
	}
	permissions: contents: "read"
	concurrency: {
		group:                "release-${{ github.repository }}-${{ inputs.tag }}-${{ inputs.runner }}"
		"cancel-in-progress": false
	}
	jobs: {
		"release-assets": {
			name:              "Verify published release artifacts"
			"runs-on":         "${{ inputs.runner }}"
			"timeout-minutes": 25
			defaults: run: shell: "pwsh"
			env: {
				MISE_ENABLE_TOOLS:            "go,gh"
				GH_TOKEN:                     "${{ github.token }}"
				CI_COMMIT_TAG:                "${{ inputs.tag }}"
				AIGW_RELEASE_ARTIFACT_SIGNER: "${{ vars.AIGW_RELEASE_ARTIFACT_SIGNER }}"
			}
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Check release admission", run: commands.readiness},
				{
					name: "Materialize provenance trust input"
					env: AIGW_RELEASE_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}"
					run: "mise exec --locked -- go run ./tools/ci trust-input --output \"$env:RUNNER_TEMP/aigw-allowed-signers\" --github-env \"$env:GITHUB_ENV\""
				},
				{
					name: "Materialize artifact signature trust"
					env: AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS }}"
					run: "mise exec --locked -- go run ./tools/ci trust-input --artifact --output \"$env:RUNNER_TEMP/aigw-artifact-signers\" --github-env \"$env:GITHUB_ENV\""
				},
				{
					name: "Download this peer's published artifacts"
					run:  #"mise exec --locked -- gh release download "$env:CI_COMMIT_TAG" --repo "$env:GITHUB_REPOSITORY" --dir dist"#
				},
				{name: "Verify complete artifacts and signed source", run: commands.artifacts},
				{
					name: "Verify published native lifecycle"
					if:   "inputs.native_lifecycle"
					env: {
						AIGW_VERIFY_SYSTEM_KEYRING:        "${{ runner.os != 'Linux' && '1' || '0' }}"
						AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE: "ephemeral-host"
					}
					run: "mise exec --locked -- go run ./tools/release accept-native --artifacts dist"
				},
			]
		}
	}
}
