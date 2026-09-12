package ci

import "strings"

// pipeline.cue owns CI topology. Forge files are generated projections.

#OperatingSystem: "darwin" | "linux" | "windows"
#JobID:           "accepted-ref-parity" | "quality" | "native-darwin" | "native-linux" | "native-windows" | "release-readiness" | "package" | "publish" | "release"
#Claim:           "accepted-ref-parity" | "source-quality" | "go-source-compatibility" | "native-product-journey" | "lifecycle-acceptance" | "release-metadata" | "artifact-construction" | "artifact-publication" | "release-record"

#Job: {
	stage: "verify" | "package" | "publish" | "release"
	rank:  int & >=0
	needs: [...#JobID]
	claims: [#Claim, ...#Claim]
}

commands: {
	install:      "mise install --locked"
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
	build:     "mise exec --locked -- go run ./tools/release build-ci build/release dist"
	upload:    "mise exec --locked -- go run ./tools/release upload-gitlab dist"
	publish:   "mise exec --locked -- go run ./tools/release publish-gitlab dist"
	acceptedRefParity: {
		gitlab: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.releaseBranch)=$CI_COMMIT_SHA\" --expect \"\(lifecycle.acceptedBranch)=$CI_COMMIT_SHA\""
		github: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.releaseBranch)=${{ github.sha }}\" --expect \"\(lifecycle.acceptedBranch)=${{ github.sha }}\""
	}
}

goToolchain: MISE_ENABLE_TOOLS:      "go"
nativeToolchain: MISE_ENABLE_TOOLS:  "\(qualityToolchain.MISE_ENABLE_TOOLS),glab,github:anchore/syft"
qualityToolchain: MISE_ENABLE_TOOLS: "go,node,cue,github:boyter/scc,github:editorconfig-checker/editorconfig-checker,github:gitleaks/gitleaks,github:golangci/golangci-lint,github:goreleaser/goreleaser,go:github.com/google/osv-scanner/v2/cmd/osv-scanner,github:lycheeverse/lychee,github:rhysd/actionlint,taplo"

#MiseConfiguration: {
	_root:                   string
	MISE_CONFIG_DIR:         "\(_root)/.config/ci"
	MISE_GLOBAL_CONFIG_FILE: "\(_root)/.config/ci/config.toml"
	MISE_SYSTEM_CONFIG_DIR:  "\(_root)/.config/ci"
}

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

// Evidence is never reused across runs. The only admitted reuse is the exact
// package artifact inside one pipeline; any identity dimension change starts
// a new proving run.
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
	samePipeline: [{
		producer: "package"
		consumers: ["publish", "release"]
		identity: "artifact-digest"
	}]
}

graph: {
	[#JobID]: #Job
	"accepted-ref-parity": {stage: "verify", rank: 0, needs: [], claims: ["accepted-ref-parity"]}
	quality: {stage: "verify", rank: 0, needs: [], claims: ["source-quality"]}
	"native-darwin": {stage: "verify", rank: 0, needs: [], claims: ["go-source-compatibility", "native-product-journey", "lifecycle-acceptance"]}
	"native-linux": {stage: "verify", rank: 0, needs: [], claims: ["go-source-compatibility", "native-product-journey", "lifecycle-acceptance"]}
	"native-windows": {stage: "verify", rank: 0, needs: [], claims: ["go-source-compatibility", "native-product-journey", "lifecycle-acceptance"]}
	"release-readiness": {stage: "verify", rank: 0, needs: [], claims: ["release-metadata"]}
	package: {
		stage: "package"
		rank:  1
		needs: ["quality", "native-darwin", "native-linux", "release-readiness"]
		claims: ["artifact-construction"]
	}
	publish: {stage: "publish", rank: 2, needs: ["package"], claims: ["artifact-publication"]}
	release: {stage: "release", rank: 3, needs: ["publish"], claims: ["release-record"]}
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

githubFullVerificationCondition: "github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || github.ref_name == '\(lifecycle.acceptedBranch)' || github.ref_name == '\(lifecycle.releaseBranch)'"

githubCommitBase: "${{ github.event.pull_request.base.sha || (github.ref_type == 'tag' && format('{0}^', github.sha)) || github.event.before || inputs.commit_base }}"

_graphOrder: {
	for id, job in graph {
		for dependency in job.needs {
			"\(id) after \(dependency)": graph[dependency].rank < job.rank
		}
	}
}

miseImage: "ghcr.io/jdx/mise:2026.9.5@sha256:d549958171c177f113e62ddba5afdfb9e245d699ac457148e4b7b0da7af3b0b7"

#MiseGitLabImage: {
	name: miseImage
	entrypoint: [""]
}

actions: {
	checkout: "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"        // v7.0.1
	mise:     "jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c"         // v4.3.0
	upload:   "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a" // v7.0.1
	sshAgent: "webfactory/ssh-agent@e83874834305fe9a4a2997156cb26c5de65a8555"    // v0.10.0
}

#ReleaseTrustFiles: {
	AIGW_RELEASE_ALLOWED_SIGNERS_FILE:          "$AIGW_RELEASE_ALLOWED_SIGNERS"
	AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE: "$AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS"
	SSH_ASKPASS_REQUIRE:                        "never"
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
		{
			name: "Run native \(nativeEvidence[_platform].name) acceptance"
			if _platform != "linux" {
				env: _credentialEnvironment
			}
			run: commands.native[_platform]
		},
		{
			name:  "Run historical release acceptance"
			if:    "github.event_name == 'workflow_dispatch' && (inputs.baseline_tag != '' || inputs.windows_clients)"
			shell: "pwsh"
			env: _credentialEnvironment & {
				GH_TOKEN:                     "${{ github.token }}"
				AIGW_BASELINE_TAG:            "${{ inputs.baseline_tag }}"
				AIGW_QUALIFY_WINDOWS_CLIENTS: "${{ inputs.windows_clients }}"
			}
			run: #"""
				$ErrorActionPreference = 'Stop'
				$PSNativeCommandUseErrorActionPreference = $true
				if ([string]::IsNullOrWhiteSpace($env:AIGW_BASELINE_TAG)) { throw 'Client qualification requires baseline_tag' }
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
				  mise exec --locked -- go run ./tools/release @acceptance
				} finally {
				  foreach ($name in @('AIGW_ACCEPTANCE_BASELINE', 'AIGW_ACCEPTANCE_CODEX', 'AIGW_ACCEPTANCE_CLAUDE', 'AIGW_ACCEPTANCE_CLIENT_PATH', 'CLAUDE_CODE_GIT_BASH_PATH')) {
				    Remove-Item "Env:$name" -ErrorAction SilentlyContinue
				  }
				  Remove-Item -LiteralPath $scope -Recurse -Force
				}
				"""#
		},
	]
}

#NativeGitLabJob: {
	_platform:     #OperatingSystem
	_refreshLocks: string
	if _platform == "windows" {
		_refreshLocks: "if ($env:AIGW_REFRESH_LOCKS -eq 'true') { \(commands.resolveLocks) }"
	}
	if _platform != "windows" {
		_refreshLocks: "if [ \"${AIGW_REFRESH_LOCKS:-false}\" = true ]; then \(commands.resolveLocks); fi"
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
		script: [commands.bootstrap, _refreshLocks, commands.native.linux]
	}
	if _platform != "linux" {
		script: [commands.install, commands.bootstrap, _refreshLocks, commands.native[_platform]]
	}
}

gitlab: {
	variables: {
		#MiseConfiguration & {_root: "$CI_PROJECT_DIR"}
		GIT_DEPTH: "0"
		GOPROXY:   "https://goproxy.cn|https://proxy.golang.org|direct"
	}
	workflow: rules: gitlabFullVerificationRules
	stages: ["verify", "package", "publish", "release"]
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
	package: {
		stage:     graph.package.stage
		tags:      nativeEvidence.darwin.gitlab.tags
		variables: #ReleaseTrustFiles
		rules: [
			{if: "$CI_COMMIT_TAG"},
			{when: "never"},
		]
		needs: [
			{job: "quality"},
			{job: "native-darwin"},
			{job: "native-linux"},
			{job: "release-readiness"},
		]
		script: [
			#"test -s "$AIGW_RELEASE_ALLOWED_SIGNERS_FILE" && test -s "$AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE" && test -n "$AIGW_RELEASE_ARTIFACT_SIGNER""#,
			#"ssh-keygen -y -P '' -f "$AIGW_RELEASE_SIGNING_KEY" >/dev/null"#,
			commands.build,
		]
		artifacts: {
			"expire_in": "30 days"
			paths: ["dist/"]
		}
	}
	publish: {
		stage:     graph.publish.stage
		tags:      nativeEvidence.darwin.gitlab.tags
		variables: #ReleaseTrustFiles
		rules: [
			{if: "$CI_COMMIT_TAG"},
			{when: "never"},
		]
		needs: [{job: "package", artifacts: true}]
		script: [commands.upload]
	}
	release: {
		stage:     graph.release.stage
		tags:      nativeEvidence.darwin.gitlab.tags
		variables: #ReleaseTrustFiles
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
		#MiseConfiguration & {_root: "${{ github.workspace }}"}
		GIT_CONFIG_COUNT:   "1"
		GIT_CONFIG_KEY_0:   "init.defaultBranch"
		GIT_CONFIG_VALUE_0: "main"
	}
	"on": {
		push: branches: [lifecycle.acceptedBranch, lifecycle.releaseBranch]
		"pull_request": branches: [lifecycle.acceptedBranch, lifecycle.releaseBranch]
		"workflow_dispatch": inputs: {
			windows_clients: {
				description: "With baseline_tag, qualify real Windows clients through the existing release lifecycle"
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

#ReleaseNativeJob: {
	_platform:         #OperatingSystem
	name:              "Native \(nativeEvidence[_platform].name) release acceptance"
	"runs-on":         nativeEvidence[_platform].github.runner
	"timeout-minutes": 25
	env:               nativeToolchain
	steps: [
		#ReleaseCheckout,
		#Toolchain,
		{name: "Prepare locked dependencies", run: commands.bootstrap},
		{name: "Run native \(nativeEvidence[_platform].name) release acceptance", run: commands.native[_platform]},
	]
}

githubRelease: {
	name: "Release"
	env: {
		#MiseConfiguration & {_root: "${{ github.workspace }}"}
		GIT_CONFIG_COUNT:   "1"
		GIT_CONFIG_KEY_0:   "init.defaultBranch"
		GIT_CONFIG_VALUE_0: "main"
	}
	"on": {
		push: tags: ["v*"]
		"workflow_dispatch": inputs: tag: {
			description: "Existing v* tag to publish"
			required:    true
			type:        "string"
		}
	}
	permissions: contents: "read"
	concurrency: {
		group:                "release-${{ github.repository }}-${{ inputs.tag || github.ref_name }}"
		"cancel-in-progress": false
	}
	jobs: {
		"native-darwin": #ReleaseNativeJob & {_platform: "darwin"}
		"native-linux": #ReleaseNativeJob & {_platform: "linux"}
		"native-windows": #ReleaseNativeJob & {_platform: "windows"}
		"package-and-publish": {
			name:        "Package and publish independently"
			"runs-on":   nativeEvidence.linux.github.runner
			environment: "release"
			permissions: contents: "write"
			env: {
				SSH_ASKPASS_REQUIRE:          "never"
				AIGW_RELEASE_ARTIFACT_SIGNER: "${{ vars.AIGW_RELEASE_ARTIFACT_SIGNER }}"
			}
			needs: ["native-darwin", "native-linux", "native-windows"]
			"timeout-minutes": 45
			steps: [
				#ReleaseCheckout,
				#Toolchain,
				{
					name: "Check release admission"
					env: CI_COMMIT_TAG: "${{ inputs.tag || github.ref_name }}"
					run: commands.readiness
				},
				{name: "Prepare locked dependencies", run: commands.bootstrap},
				{
					name: "Materialize provenance trust input"
					env: AIGW_RELEASE_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}"
					run: "mise exec --locked -- go run ./tools/ci trust-input --output \"$RUNNER_TEMP/aigw-allowed-signers\" --github-env \"$GITHUB_ENV\""
				},
				{
					name: "Verify the signed tag and source"
					env: {
						SELECTED_TAG:                      "${{ inputs.tag || github.ref_name }}"
						AIGW_COMMIT_BASE:                  "${{ format('{0}^', inputs.tag || github.ref_name) }}"
						AIGW_CHANGELOG_RELEASE_TAG:        "${{ inputs.tag || github.ref_name }}"
						AIGW_RELEASE_AUTHOR_EMAIL:         "${{ vars.AIGW_RELEASE_AUTHOR_EMAIL }}"
						AIGW_RELEASE_ALLOWED_SIGNERS_FILE: "${{ env.AIGW_RELEASE_ALLOWED_SIGNERS_FILE }}"
					}
					run: commands.source
				},
				{
					name: "Materialize artifact signature trust"
					env: AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS }}"
					run: "mise exec --locked -- go run ./tools/ci trust-input --artifact --output \"$RUNNER_TEMP/aigw-artifact-signers\" --github-env \"$GITHUB_ENV\""
				},
				{
					name: "Load the dedicated release signing key"
					uses: actions.sshAgent
					with: {
						"ssh-private-key": "${{ secrets.AIGW_RELEASE_SIGNING_PRIVATE_KEY }}"
						"log-public-key":  false
					}
				},
				{
					name: "Verify release signing capability"
					env: AIGW_RELEASE_SIGNING_PUBLIC_KEY: "${{ vars.AIGW_RELEASE_SIGNING_PUBLIC_KEY }}"
					run: #"""
						test -n "$AIGW_RELEASE_ARTIFACT_SIGNER"
						test -n "$AIGW_RELEASE_SIGNING_PUBLIC_KEY"
						key="$RUNNER_TEMP/aigw-release-signing.pub"
						printf '%s\n' "$AIGW_RELEASE_SIGNING_PUBLIC_KEY" > "$key"
						ssh-add -T "$key"
						printf 'AIGW_RELEASE_SIGNING_KEY=%s\n' "$key" >> "$GITHUB_ENV"
						"""#
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
