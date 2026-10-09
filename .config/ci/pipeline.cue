package ci

import (
	"list"
	"strings"
)

// pipeline.cue owns CI topology. Forge files are generated projections.

#OperatingSystem: "darwin" | "linux" | "windows"
#ToolSource:      "upstream" | "peer"
#JobID:           "accepted-ref-parity" | "quality" | "native-darwin" | "native-linux" | "native-windows" | "linux-secret-service" | "release-version" | "release-assets"
#Claim:           "accepted-ref-parity" | "source-quality" | "go-source-compatibility" | "native-product-journey" | "lifecycle-acceptance" | "linux-secret-service" | "release-metadata" | "artifact-verification"

#Job: {
	name:  string
	stage: "verify" | "release"
	rank:  int & >=0
	needs: [...#JobID]
	claims: [#Claim, ...#Claim]
}

// Go's module and checksum hosts reset HTTP/2 streams during cold installs.
// Keep this transport choice in the installer process, not product execution.
installationEnvironment: GODEBUG: "http2client=0"

// Git source acquisition must admit the same paths as native tool caches.
gitEnvironment: {
	GIT_CONFIG_COUNT:   "2"
	GIT_CONFIG_KEY_0:   "init.defaultBranch"
	GIT_CONFIG_VALUE_0: "main"
	GIT_CONFIG_KEY_1:   "core.longpaths"
	GIT_CONFIG_VALUE_1: "true"
}

toolSourceInput: {
	description: "Locked upstream distribution or the selected peer's verified immutable tool copies"
	required:    false
	type:        "choice"
	default:     #ToolSource & "upstream"
	options: ["upstream", "peer"]
}

// Peer-local copies transport only the upstream bytes selected by mise.lock.
// They do not own dependencies or checksums; missing copies fail locally.
miseMirror: {
	package:          "mise-github"
	resource:         "packages/generic/\(package)/"
	metadataPattern:  "regex:^https://api[.]github[.]com/repos/([^/]+)/([^/]+)/releases/tags/([^/?]+)$"
	metadataResource: "release-$1-$2-$3.json"
	unixDirectory:    "$(pwd -P)/build/tmp/.aigw-mise-mirror-$CI_JOB_ID"
	unixPrepare:      #"""
		set -eu
		case "${AIGW_TOOL_SOURCE:-upstream}" in
		  upstream) ;;
		  peer)
		: "${CI_API_V4_URL:?}"
		: "${CI_PROJECT_ID:?}"
		: "${CI_SERVER_HOST:?}"
		: "${CI_JOB_ID:?}"
		: "${CI_JOB_TOKEN:?}"
		mirror_dir="\#(unixDirectory)"
		mkdir -p -m 700 "$mirror_dir"
		if command -v sha256sum >/dev/null 2>&1; then
		  lock_digest="$(sha256sum mise.lock)"
		else
		  lock_digest="$(shasum -a 256 mise.lock)"
		fi
		lock_digest="${lock_digest%% *}"
		export MISE_DATA_DIR="$mirror_dir/mise-data"
		export MISE_CACHE_DIR="$MISE_DATA_DIR/cache"
		(umask 077; printf 'machine %s login gitlab-ci-token password %s\n' "$CI_SERVER_HOST" "$CI_JOB_TOKEN" > "$mirror_dir/netrc")
		export MISE_NETRC_FILE="$mirror_dir/netrc"
		export MISE_NETRC=1
		mirror_base="$CI_API_V4_URL/projects/$CI_PROJECT_ID/\#(resource)$lock_digest/"
		export MISE_URL_REPLACEMENTS="$(printf '{"\#(metadataPattern)":"%s\#(metadataResource)","https://github.com/":"%s","https://api.github.com/":"%s"}' "$mirror_base" "$mirror_base" "$mirror_base")"
		    ;;
		  *) printf '%s\n' 'AIGW_TOOL_SOURCE must be upstream or peer' >&2; exit 1 ;;
		esac
		"""#
	unixCleanup:      #"""
		set -eu
		if [ -n "${CI_JOB_ID:-}" ]; then
		  mirror_dir="\#(unixDirectory)"
		  rm -rf -- "$mirror_dir"
		  if [ -e "$mirror_dir" ] || [ -L "$mirror_dir" ]; then
		    printf '%s\n' 'Mise job-owned supply state remains after cleanup.' >&2
		    exit 1
		  fi
		  printf '%s\n' 'Mise job-owned supply state retired.'
		fi
		"""#
	githubEnvironment: MISE_URL_REPLACEMENTS: "{{\"regex:^https://gitlab[.]com/gitlab-org/cli/-/releases/([^/]+)/downloads/([^/?]+)$\":\"{0}/{1}/releases/download/mise-glab-$1/$2\",\"regex:^https://gitlab[.]com/api/v4/projects/gitlab-org%2Fcli/packages/generic/glab/([^/]+)/([^/?]+)$\":\"{0}/{1}/releases/download/mise-glab-v$1/$2\"}}"
}

linuxApt: {
	deadline: "timeout --verbose --kill-after=5s 240s"
	options:  "-o Acquire::Retries=1 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30"
	update:   "\(deadline) apt-get \(options) update"
	install:  "\(deadline) apt-get \(options) install --no-install-recommends -y"
}

linuxToolchain: {
	// The runnable Mise image is intentionally small. Declare the complete
	// repository execution closure here so every Linux job inherits one owner.
	runtimePackages: ["libatomic1", "openssh-client", "procps", "time"]
	prepare:  "set -eu\n\(linuxApt.update)\nDEBIAN_FRONTEND=noninteractive \(linuxApt.install) \(strings.Join(runtimePackages, " "))"
	compiler: "set -eu\nDEBIAN_FRONTEND=noninteractive \(linuxApt.install) gcc libc6-dev"
}

linuxSecretService: {
	packages: "dbus-x11 gnome-keyring libglib2.0-bin"
	session: {
		command: string
		// A private session isolates native state without changing vendor diagnostics.
		run: #"""
			dbus-run-session -- bash -s -euo pipefail -- "$@" <<'AIGW_SECRET_SERVICE'
			gdbus call --session --dest org.freedesktop.secrets --object-path /org/freedesktop/secrets --method org.freedesktop.Secret.Service.ReadAlias session | grep -Fq /org/freedesktop/secrets/collection/session
			gdbus call --session --dest org.freedesktop.secrets --object-path /org/freedesktop/secrets --method org.freedesktop.Secret.Service.SetAlias default /org/freedesktop/secrets/collection/session >/dev/null
			AIGW_VERIFY_SYSTEM_KEYRING=1 \#(command)
			AIGW_SECRET_SERVICE
			"""#
	}
	journey: (session & {command: "mise exec --locked -- go test ./tools/release -run \"^TestNativeProductJourney/system_credential_store$\" -count=1 -v"}).run
	githubPrepare: "sudo -n \(linuxApt.update)\nsudo -n DEBIAN_FRONTEND=noninteractive \(linuxApt.install) \(packages)"
	github:        "\(githubPrepare)\n\(journey)"
	// The GitLab Mise image is root-owned; its shared before_script updates apt.
	gitlab: "DEBIAN_FRONTEND=noninteractive \(linuxApt.install) \(packages)\n\(journey)"
}

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
	version:   "mise exec --locked -- go run ./tools/release validate-version-tag"
	artifacts: "mise exec --locked -- go run ./tools/release verify-artifacts dist"
	acceptedRefParity: {
		gitlab: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.releaseBranch)=$CI_COMMIT_SHA\" --expect \"\(lifecycle.acceptedBranch)=$CI_COMMIT_SHA\""
		github: "mise exec --locked -- go run ./tools/forge refs --remote \(lifecycle.checkoutRemote) --expect \"\(lifecycle.releaseBranch)=${{ github.sha }}\" --expect \"\(lifecycle.acceptedBranch)=${{ github.sha }}\""
	}
}

toolchainTools: {
	bootstrap: ["go", "node", "npm"]
	performance: ["github:sharkdp/hyperfine"]
	portableQuality: list.Concat([bootstrap, [
		"cue",
		"github:boyter/scc",
		"github:editorconfig-checker/editorconfig-checker",
		"github:gitleaks/gitleaks",
		"github:golangci/golangci-lint",
		"github:goreleaser/goreleaser",
		"github:google/osv-scanner",
		"github:rhysd/actionlint",
	], performance, [
		"shellcheck",
		"taplo",
		"typos",
	]])
	links: ["github:lycheeverse/lychee"]
	quality: list.Concat([portableQuality, links])
	// Native Go suites execute real glab against disposable GitLab origins.
	native: list.Concat([quality, ["github:anchore/syft", "gh", "glab"]])
	nativeArtifact: ["go", "gh", "glab", "github:goreleaser/goreleaser"]
	secretService: ["go", "github:goreleaser/goreleaser"]
	darwin: ["github:indygreg/apple-platform-rs"]
}

goToolchain: MISE_ENABLE_TOOLS:      "go"
qualityToolchain: MISE_ENABLE_TOOLS: strings.Join(toolchainTools.quality, ",")
nativeToolchain: {
	for platform in ["darwin", "linux", "windows"] {
		(platform): MISE_ENABLE_TOOLS: strings.Join(list.Concat([
			toolchainTools.native,
			if platform == "darwin" {toolchainTools.darwin},
			if platform != "darwin" {[]},
		]), ",")
	}
}

nativeArtifactToolchain: {
	for platform in ["darwin", "linux", "windows"] {
		(platform): MISE_ENABLE_TOOLS: strings.Join(list.Concat([
			toolchainTools.nativeArtifact,
			if platform == "darwin" {toolchainTools.darwin},
			if platform != "darwin" {[]},
		]), ",")
	}
}

// Git role names belong to the adopter workspace; CUE consumes its native TOML.
branch_roles: {accepted_branch: string, release_branch: string}

lifecycle: {
	releaseBranch:  branch_roles.release_branch
	acceptedBranch: branch_roles.accepted_branch
	checkoutRemote: "origin"
}

// Product evidence is declared once; each Forge projects the complete matrix.
productEvidence: native: ["darwin", "linux", "windows"]

gitlabControlPlatform: #OperatingSystem & "darwin"

// This map owns native execution evidence only. Product release targets remain
// solely owned by .config/release/goreleaser.yaml.
nativeEvidence: {
	darwin: {
		name: "macOS"
		gitlab: {
			protectedTag: "ci-macos-arm64-shell"
			tags: [protectedTag]
			reviewTag: "ci-macos-arm64-review"
		}
		github: runner: "macos-26-intel"
	}
	linux: {
		name: "Linux"
		gitlab: {
			protectedTag: "ci-linux-arm64-container-protected"
			reviewTag:    "ci-linux-arm64-container"
			tags: ["$AIGW_CI_LINUX_RUNNER_TAG"]
		}
		github: runner: "ubuntu-24.04"
	}
	windows: {
		name: "Windows"
		gitlab: {
			protectedTag: "ci-windows-arm64-shell"
			tags: [protectedTag]
			reviewTag: "ci-windows-arm64-review"
		}
		github: runner: "windows-2025"
	}
}

graph: {
	[#JobID]: #Job
	"accepted-ref-parity": {name: "Accepted ref parity", stage: "verify", rank: 0, needs: [], claims: ["accepted-ref-parity"]}
	quality: {name: "Quality and governance", stage: "verify", rank: 0, needs: [], claims: ["source-quality"]}
	for platform in productEvidence.native {
		"native-\(platform)": {name: "Native \(nativeEvidence[platform].name) acceptance", stage: "verify", rank: 0, needs: [], claims: ["go-source-compatibility", "native-product-journey", "lifecycle-acceptance"]}
	}
	"linux-secret-service": {name: "Linux Secret Service", stage: "verify", rank: 0, needs: [], claims: ["linux-secret-service"]}
	"release-version": {name: "Release version", stage: "verify", rank: 0, needs: [], claims: ["release-metadata"]}
	"release-assets": {name: "Verify published release artifacts", stage: "release", rank: 1, needs: list.Concat([["quality"], [for platform in productEvidence.native {"native-\(platform)"}], ["linux-secret-service", "release-version"]]), claims: ["artifact-verification"]}
}

// The same declared release dependencies name the GitHub tag jobs; the
// read-only Release workflow must not substitute another peer or attempt.
githubTagEvidenceJobs: strings.Join([
	for dependency in graph["release-assets"].needs {
		"--job '\(graph[dependency].name)'"
	},
], " ")

gitlabVerificationCondition: {
	tag: "$CI_COMMIT_TAG"
	// A fork MR run in the parent has a different source project ID.
	review:        "$CI_PIPELINE_SOURCE == \"merge_request_event\" && ($CI_MERGE_REQUEST_TARGET_BRANCH_NAME == \"\(lifecycle.acceptedBranch)\" || $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == \"\(lifecycle.releaseBranch)\") && $CI_MERGE_REQUEST_SOURCE_PROJECT_ID == $CI_PROJECT_ID"
	protectedPush: "$CI_PIPELINE_SOURCE == \"push\" && ($CI_COMMIT_BRANCH == \"\(lifecycle.acceptedBranch)\" || $CI_COMMIT_BRANCH == \"\(lifecycle.releaseBranch)\")"
	manual:        "$CI_PIPELINE_SOURCE == \"web\" || $CI_PIPELINE_SOURCE == \"api\""
	manualSource:  "($CI_PIPELINE_SOURCE == \"web\" || $CI_PIPELINE_SOURCE == \"api\") && (($AIGW_NATIVE_INPUT_PACKAGE == null || $AIGW_NATIVE_INPUT_PACKAGE == \"\") && ($AIGW_CANDIDATE_ARTIFACTS == null || $AIGW_CANDIDATE_ARTIFACTS == \"\") && ($AIGW_CANDIDATE_TAG == null || $AIGW_CANDIDATE_TAG == \"\") || $AIGW_FULL_NATIVE_QUALITY == \"true\" || $AIGW_REFRESH_LOCKS == \"true\")"
}

gitlabPipelineRules: [
	{
		if: gitlabVerificationCondition.tag
		auto_cancel: on_new_commit:          "none"
		variables: AIGW_CI_LINUX_RUNNER_TAG: nativeEvidence.linux.gitlab.protectedTag
	},
	{
		if: gitlabVerificationCondition.review
		variables: AIGW_CI_LINUX_RUNNER_TAG: nativeEvidence.linux.gitlab.reviewTag
	},
	{
		if: gitlabVerificationCondition.protectedPush
		auto_cancel: on_new_commit:          "none"
		variables: AIGW_CI_LINUX_RUNNER_TAG: nativeEvidence.linux.gitlab.protectedTag
	},
	for protected, tag in {true: nativeEvidence.linux.gitlab.protectedTag, false: nativeEvidence.linux.gitlab.reviewTag} {
		{
			if: "(\(gitlabVerificationCondition.manual)) && $CI_COMMIT_REF_PROTECTED == \"\(protected)\""
			auto_cancel: on_new_commit:          "none"
			variables: AIGW_CI_LINUX_RUNNER_TAG: tag
		}
	},
	{when: "never"},
]

githubFullVerificationCondition:   "github.ref_type == 'tag' || github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || (github.event_name == 'push' && (github.ref_name == '\(lifecycle.acceptedBranch)' || github.ref_name == '\(lifecycle.releaseBranch)'))"
githubSourceVerificationCondition: "(\(githubFullVerificationCondition)) && (github.event_name != 'workflow_dispatch' || github.ref_type == 'tag' || inputs.full_quality || inputs.refresh_locks || inputs.windows_clients || (inputs.candidate_tag == '' && inputs.input_release == ''))"

githubCommitBase: "${{ github.event.pull_request.base.sha || (github.ref_type == 'tag' && format('{0}^', github.sha)) || github.event.before || inputs.commit_base || format('{0}^', github.sha) }}"

_graphOrder: {
	for id, job in graph {
		for dependency in job.needs {
			"\(id) after \(dependency)": graph[dependency].rank < job.rank
		}
	}
}

miseImage:                        "docker.io/jdxcode/mise:2026.10.3-debian@sha256:58c4c847f5518a9a87a9a582886a485443dbca5eb024426577f58d586a0990c6"
miseVersion:                      strings.TrimSuffix(strings.Split(strings.Split(miseImage, ":")[1], "@")[0], "-debian")
miseWindowsArm64ExecutableSHA256: "f307609491da1cb78ee3fd2de8949e7cce1a7e3032e1c1671dbfdfdf304ba2e1"
miseWindowsArm64ShimSHA256:       "b9dff021fa072a116cb56940728a391dccd337a15e70100690c2bf58f85bbdab"
windowsMiseJobDirectory:          "Join-Path (Split-Path -Parent $env:CI_PROJECT_DIR) \"aigw-ci-mise-$env:CI_JOB_ID\""
nativeWindowsClientSelection:     "$env:AIGW_NATIVE_CLIENTS -eq 'true' -or -not [string]::IsNullOrWhiteSpace($env:AIGW_NATIVE_DIAGNOSTIC_CLIENT)"

// Windows owns official client supply only; the portable release owner acquires
// and admits the candidate and predecessor matrices on every native platform.
nativeWindowsClientSupply: #"""
	if ($env:AIGW_NATIVE_INPUT_PACKAGE -and (\#(nativeWindowsClientSelection))) {
	  if ($env:AIGW_NATIVE_PLATFORM -ne 'windows') { throw 'Native public input requires the Windows platform' }
	  if ($env:AIGW_NATIVE_INPUT_SHA256 -notmatch '^[0-9a-f]{64}$' -or $env:AIGW_CANDIDATE_SOURCE -notmatch '^[0-9a-f]{40}$') { throw 'Native public input requires exact source and checksum.' }
	  if (-not $env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE) { $env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE = $env:AIGW_RELEASE_ALLOWED_SIGNERS }
	  if (-not $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE) { $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE = $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS }
	  if ([string]::IsNullOrWhiteSpace($env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE) -or [string]::IsNullOrWhiteSpace($env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE) -or [string]::IsNullOrWhiteSpace($env:AIGW_RELEASE_ARTIFACT_SIGNER) -or -not (Test-Path -LiteralPath $env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE -PathType Leaf) -or -not (Test-Path -LiteralPath $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE -PathType Leaf)) { throw 'Native public input requires configured source and artifact trust' }
	  $jobDirectory = \#(windowsMiseJobDirectory)
	  $archive = Join-Path $jobDirectory 'native-input.tar'
	  $fixture = Join-Path $jobDirectory 'native-input'
	  $previousLogin = $env:GLAB_ENABLE_CI_AUTOLOGIN
	  $previousConfig = $env:GLAB_CONFIG_DIR
	  try {
	    $env:GLAB_ENABLE_CI_AUTOLOGIN = 'true'
	    $env:GLAB_CONFIG_DIR = Join-Path $jobDirectory 'glab'
	    mise exec --locked -- glab packages download --repo $env:CI_PROJECT_URL --name $env:AIGW_NATIVE_INPUT_PACKAGE --version $env:AIGW_CANDIDATE_SOURCE --filename public-inputs.tar --path $archive
	  } finally {
	    $env:GLAB_CONFIG_DIR = $previousConfig
	    $env:GLAB_ENABLE_CI_AUTOLOGIN = $previousLogin
	  }
	  if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $env:AIGW_NATIVE_INPUT_SHA256) { throw 'Native public input checksum mismatch' }
	  [void](New-Item -ItemType Directory -Path $fixture -ErrorAction Stop)
	  tar -xf $archive -C $fixture suppliers/
	  tar -xf (Join-Path $fixture 'suppliers/windows-codex-0.160.1-claude-2.1.292-native-packages.tar.gz') -C $fixture
	  $hermes = Join-Path $fixture 'clients/hermes'
	  [void](New-Item -ItemType Directory -Path $hermes -ErrorAction Stop)
	  tar -xf (Join-Path $fixture 'suppliers/official-hermes-f97608f1-source.tar') -C $hermes
	  $env:UV_CACHE_DIR = Join-Path $jobDirectory 'uv-cache'
	  $env:UV_PYTHON_INSTALL_DIR = Join-Path $jobDirectory 'python'
	  $env:UV_PROJECT_ENVIRONMENT = Join-Path $hermes '.venv'
	  $env:UV_PYTHON_INSTALL_REGISTRY = '0'
	  $env:UV_PYTHON_INSTALL_BIN = '0'
	  $env:UV_NO_CONFIG = '1'
	  mise exec --locked -- uv sync --project $hermes --python 3.12 --managed-python --frozen --no-dev --no-default-groups --extra edge-tts --extra bedrock
	  $codex = Join-Path $fixture 'clients/node_modules/@openai/codex/vendor/x86_64-pc-windows-msvc'
	  $gitBin = Split-Path (Get-Command git.exe -ErrorAction Stop).Source
	  $bash = Join-Path (Split-Path $gitBin) 'bin/bash.exe'
	  if (-not (Test-Path -LiteralPath $bash -PathType Leaf)) { throw 'Native Claude requires Git Bash.' }
	  $env:AIGW_ACCEPTANCE_CODEX = Join-Path $codex 'bin/codex.exe'
	  $env:AIGW_ACCEPTANCE_CLAUDE = Join-Path $fixture 'clients/node_modules/@anthropic-ai/claude-code-win32-x64/claude.exe'
	  $env:AIGW_ACCEPTANCE_HERMES = Join-Path $hermes '.venv/Scripts/hermes.exe'
	  $node = mise which node
	  $env:AIGW_ACCEPTANCE_CLIENT_PATH = @((Join-Path $codex 'codex-path'), (Join-Path $hermes '.venv/Scripts'), (Split-Path $node), $gitBin, (Split-Path $bash), (Join-Path $env:SystemRoot 'System32')) -join ';'
	  $env:CLAUDE_CODE_GIT_BASH_PATH = $bash
	}
	"""#

actions: {
	checkout: "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"        // v7.0.1
	mise:     "jdx/mise-action@2d8d4cafcbd33be2ea37d2b6f5ad595363d1f1ca"         // v5.1.1
	upload:   "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a" // v7.0.1
}

dependencyEvidencePath: "build/verification/dependencies"
workflowEvidencePath:   "build/verification/workflows"

#DependencyEvidenceGitHubStep: {
	_name: string
	name:  "Retain native dependency evidence"
	if:    "always()"
	uses:  actions.upload
	with: {
		name:                _name
		path:                dependencyEvidencePath
		"if-no-files-found": "ignore"
	}
}

hermesSourceCommit:    "f97608f178d1ffeca59860195ab7da295f7c8e5f"
hermesInstallerDigest: "0a80dfeb7434229933bac32e73140d10086dff81bd84b156e71be9abc87cddf2"

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
	env: installationEnvironment & {
		MISE_URL_REPLACEMENTS: "${{ inputs.tool_source == 'peer' && format('\(miseMirror.githubEnvironment.MISE_URL_REPLACEMENTS)', github.server_url, github.repository) || '' }}"
	}
	with: {
		version:          miseVersion
		install:          true
		install_args:     "--locked"
		cache:            "${{ inputs.tool_source != 'peer' }}"
		cache_key_prefix: "mise-${{ github.job }}"
	}
}

#SourceEvidenceGitHubStep: {
	_name: string
	name:  "Retain native source evidence"
	if:    "always()"
	uses:  actions.upload
	with: {name: _name, path: "\(workflowEvidencePath)\nbuild/verification/coverage", "if-no-files-found": "ignore"}
}

#NativeGitHubJob: {
	_platform:            #OperatingSystem
	_sourceCondition:     "github.event_name != 'workflow_dispatch' || inputs.full_quality || inputs.refresh_locks || inputs.windows_clients || (inputs.candidate_tag == '' && inputs.input_release == '')"
	_historicalCondition: "github.event_name == 'workflow_dispatch' && (inputs.baseline_tag != '' || inputs.candidate_tag != '' || inputs.input_release != '' || inputs.windows_clients || inputs.diagnostic_client != '' || inputs.macos_keychain || inputs.performance || inputs.performance_attribution)"
	_environmentPrefix:   string
	_clients:             string
	if _platform == "windows" {
		_environmentPrefix: "$env:"
		_clients:           "${{ inputs.windows_clients && inputs.diagnostic_client == '' }}"
	}
	if _platform != "windows" {
		_environmentPrefix: "$"
		_clients:           "false"
	}
	_historicalArguments: "--peer=github --repository=\"\(_environmentPrefix)GITHUB_REPOSITORY\" --baseline-tag=\"\(_environmentPrefix)AIGW_BASELINE_TAG\" --tag=\"\(_environmentPrefix)AIGW_CANDIDATE_TAG\" --clients=\(_clients) --diagnostic-client=\"\(_environmentPrefix)AIGW_NATIVE_DIAGNOSTIC_CLIENT\" --performance-attribution=${{ inputs.performance_attribution }} --input-release=\"\(_environmentPrefix)AIGW_NATIVE_INPUT_RELEASE\" --input-sha256=\"\(_environmentPrefix)AIGW_NATIVE_INPUT_SHA256\" --candidate-source=\"\(_environmentPrefix)AIGW_CANDIDATE_SOURCE\" --candidate=${{ inputs.input_release != '' }}"
	_performanceCommand:  "mise run performance \(_historicalArguments) --performance \"\(_environmentPrefix)GITHUB_WORKSPACE/build/performance\""
	_historicalEnvironment: _credentialEnvironment & {
		if _platform == "darwin" {
			AIGW_VERIFY_SYSTEM_KEYRING: "${{ github.event_name == 'workflow_dispatch' && inputs.macos_keychain && '1' || '0' }}"
		}
		GH_TOKEN:                      "${{ github.token }}"
		GH_PROMPT_DISABLED:            "1"
		AIGW_BASELINE_TAG:             "${{ inputs.baseline_tag }}"
		AIGW_CANDIDATE_TAG:            "${{ inputs.candidate_tag }}"
		AIGW_NATIVE_DIAGNOSTIC_CLIENT: "${{ inputs.diagnostic_client }}"
		AIGW_NATIVE_INPUT_RELEASE:     "${{ inputs.input_release }}"
		AIGW_NATIVE_INPUT_SHA256:      "${{ inputs.input_sha256 }}"
		AIGW_CANDIDATE_SOURCE:         "${{ inputs.candidate_source }}"
		AIGW_RELEASE_ARTIFACT_SIGNER:  "${{ vars.AIGW_RELEASE_ARTIFACT_SIGNER }}"
	}
	_credentialEnvironment: {
		if _platform == "windows" {
			AIGW_VERIFY_SYSTEM_KEYRING: "1"
		}
		if _platform == "darwin" {
			AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE: "ephemeral-host"
		}
	}
	name:      "${{ github.event_name == 'workflow_dispatch' && 'Manual \(graph["native-\(_platform)"].name)' || '\(graph["native-\(_platform)"].name)' }}"
	"runs-on": nativeEvidence[_platform].github.runner
	if _platform != "windows" {defaults: run: shell: "bash"}
	if _platform == "windows" {defaults: run: shell: "pwsh"}
	"timeout-minutes": 25
	if:                "(\(githubFullVerificationCondition)) && (github.event_name != 'workflow_dispatch' || github.ref_type == 'tag' || inputs.native_platform == '' || inputs.native_platform == 'all' || inputs.native_platform == '\(_platform)')"
	env: MISE_ENABLE_TOOLS: "${{ (\(_sourceCondition)) && '\(nativeToolchain[_platform].MISE_ENABLE_TOOLS)' || inputs.performance && '\(nativeArtifactToolchain[_platform].MISE_ENABLE_TOOLS),\(strings.Join(toolchainTools.performance, ","))' || '\(nativeArtifactToolchain[_platform].MISE_ENABLE_TOOLS)' }}"
	steps: [
		#SourceCheckout,
		#Toolchain,
		{name: "Prepare locked dependencies", if: _sourceCondition, run: commands.bootstrap},
		if _platform == "linux" {
			name: "Prepare native Secret Service"
			if:   "\(_sourceCondition) || \(_historicalCondition)"
			run:  linuxSecretService.githubPrepare
		},
		if _platform == "linux" {
			name: "Prepare native memory measurement"
			if:   "\(_sourceCondition) || inputs.performance"
			run:  "set -eu\nsudo -n \(linuxApt.update)\nsudo -n DEBIAN_FRONTEND=noninteractive \(linuxApt.install) time"
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
				name:                   "mise-lock-\(_platform)"
				path:                   "mise.lock\n.mise/locks\n"
				"include-hidden-files": true
			}
		},
		for full in [false, true] {
			if !full {
				name: "Run native \(nativeEvidence[_platform].name) acceptance"
				if:   "github.event_name != 'workflow_dispatch' || (!inputs.full_quality && inputs.baseline_tag == '' && inputs.candidate_tag == '' && inputs.input_release == '')"
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
		// Fetch the Git blob bytes; checkout can rewrite the installer's declared CRLF worktree form.
		if _platform == "windows" {
			name:              "Fetch pinned Hermes installer"
			if:                "github.event_name == 'workflow_dispatch' && inputs.windows_clients && inputs.baseline_tag != ''"
			shell:             "pwsh"
			"timeout-minutes": 2
			env: {
				GH_TOKEN:           "${{ github.token }}"
				GH_PROMPT_DISABLED: "1"
			}
			run: #"""
				$ErrorActionPreference = 'Stop'
				$PSNativeCommandUseErrorActionPreference = $true
				$hermesCommit = '\#(hermesSourceCommit)'
				$hermesInstallerDigest = '\#(hermesInstallerDigest)'
				$metadata = mise exec --locked -- gh api "repos/NousResearch/hermes-agent/contents/scripts/install.ps1?ref=$hermesCommit" | ConvertFrom-Json
				if ($LASTEXITCODE -ne 0 -or $metadata.type -ne 'file' -or $metadata.encoding -ne 'base64') { throw 'Pinned Hermes installer metadata is unavailable' }
				$installer = Join-Path $env:RUNNER_TEMP ([guid]::NewGuid().ToString() + '.ps1')
				[IO.File]::Open($installer, [IO.FileMode]::CreateNew).Dispose()
				try {
				  [IO.File]::WriteAllBytes($installer, [Convert]::FromBase64String($metadata.content))
				  if ((Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hermesInstallerDigest) { throw 'Pinned Hermes installer checksum mismatch' }
				  Add-Content -LiteralPath $env:GITHUB_ENV -Value "AIGW_HERMES_INSTALLER=$installer"
				} catch {
				  Remove-Item -LiteralPath $installer -Force -ErrorAction Stop
				  throw
				}
				"""#
		},
		if _platform == "windows" {
			name:              "Prepare official Windows clients"
			if:                "github.event_name == 'workflow_dispatch' && inputs.windows_clients && inputs.baseline_tag != ''"
			"timeout-minutes": 12
			env: {
				GH_TOKEN:            ""
				GITHUB_TOKEN:        ""
				GIT_TERMINAL_PROMPT: "0"
			}
			run: #"""
				$ErrorActionPreference = 'Stop'
				$PSNativeCommandUseErrorActionPreference = $true
				$clients = Join-Path $env:RUNNER_TEMP ([guid]::NewGuid().ToString())
				New-Item -ItemType Directory -Path $clients | Out-Null
				$hermesCommit = '\#(hermesSourceCommit)'
				$hermesInstaller = $env:AIGW_HERMES_INSTALLER
				try {
				  Add-Content -LiteralPath $env:GITHUB_ENV -Value "AIGW_NATIVE_CLIENT_SUPPLY=$clients"
				  $env:GIT_CONFIG_GLOBAL = Join-Path $clients 'gitconfig'
				  git config --file $env:GIT_CONFIG_GLOBAL core.autocrlf false
				  if ((Get-FileHash -LiteralPath $hermesInstaller -Algorithm SHA256).Hash.ToLowerInvariant() -ne '\#(hermesInstallerDigest)') { throw 'Pinned Hermes installer checksum mismatch.' }
				  $env:UV_CACHE_DIR = Join-Path $clients 'uv-cache'
				  Set-Content -LiteralPath (Join-Path $clients 'package.json') -Value '{"private":true}'
				  mise exec --locked -- npm install --prefix $clients --ignore-scripts --save-exact --no-audit --no-fund '@openai/codex@0.160.1' '@anthropic-ai/claude-code-win32-x64@2.1.292'
				  mise exec --locked -- npm audit signatures --prefix $clients
				  $hermesHome = Join-Path $clients 'hermes'
				  $hermesInstall = Join-Path $hermesHome 'hermes-agent'
				  foreach ($stage in @('uv', 'git', 'repository', 'python', 'venv', 'dependencies')) {
				    $frames = @(& pwsh -NoProfile -File $hermesInstaller -Commit $hermesCommit -HermesHome $hermesHome -InstallDir $hermesInstall -NonInteractive -Json -Stage $stage)
				    if ($LASTEXITCODE -ne 0 -or $frames.Count -eq 0) { throw "Hermes install stage failed: $stage" }
				    $frame = $frames[-1] | ConvertFrom-Json
				    if ($frame.stage -ne $stage -or -not $frame.ok -or $frame.skipped) { throw "Hermes install stage was not admitted: $stage" }
				    if ($stage -eq 'dependencies' -and -not ($frames -match 'hash-verified via uv.lock')) { throw 'Hermes dependencies lack locked hash verification.' }
				  }
				  $observed = git -C $hermesInstall rev-parse HEAD
				  if ($LASTEXITCODE -ne 0 -or $observed.Trim() -ne $hermesCommit) { throw 'Hermes installed source differs from the pin.' }
				  $changes = git -C $hermesInstall status --porcelain=v1 --untracked-files=no
				  if ($LASTEXITCODE -ne 0 -or $changes) { throw 'Pinned Hermes source is not clean after official installation.' }
				  $codexRoot = Join-Path $clients 'node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc'
				  $gitBin = Split-Path (Get-Command git.exe).Source
				  $bash = Join-Path (Split-Path $gitBin) 'bin/bash.exe'
				  if (-not (Test-Path -LiteralPath $bash -PathType Leaf)) { throw 'Claude requires native Git Bash.' }
				  $outputs = [ordered]@{
				    AIGW_ACCEPTANCE_CODEX = Join-Path $codexRoot 'bin/codex.exe'
				    AIGW_ACCEPTANCE_CLAUDE = Join-Path $clients 'node_modules/@anthropic-ai/claude-code-win32-x64/claude.exe'
				    AIGW_ACCEPTANCE_HERMES = Join-Path $hermesInstall 'venv/Scripts/hermes.exe'
				  }
				  foreach ($executable in $outputs.Values) { Get-FileHash -LiteralPath $executable -Algorithm SHA256 }
				  $outputs.CLAUDE_CODE_GIT_BASH_PATH = $bash
				  $outputs.AIGW_ACCEPTANCE_CLIENT_PATH = @((Join-Path $codexRoot 'codex-path'), (Join-Path $hermesHome 'bin'), (Join-Path $hermesInstall 'venv/Scripts'), (Split-Path (Get-Command node.exe).Source), $gitBin, (Split-Path $bash), (Join-Path $env:SystemRoot 'System32')) -join ';'
				  foreach ($entry in $outputs.GetEnumerator()) { Add-Content -LiteralPath $env:GITHUB_ENV -Value "$($entry.Key)=$($entry.Value)" }
				} catch {
				  Remove-Item -LiteralPath $clients -Recurse -Force -ErrorAction Stop
				  throw
				}
				"""#
		},
		for trust in [{kind: "source", flag: ""}, {kind: "artifact", flag: " --artifact"}] {
			name: "Prepare native \(trust.kind) trust"
			if:   _historicalCondition
			env: {
				AIGW_RELEASE_ALLOWED_SIGNERS:          "${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}"
				AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS: "${{ vars.AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS }}"
			}
			run: "mise exec --locked -- go run ./tools/ci trust-input\(trust.flag) --output \"\(_environmentPrefix)RUNNER_TEMP/aigw-native-\(trust.kind)-signers\" --github-env \"\(_environmentPrefix)GITHUB_ENV\""
		},
		{
			name: "Run historical release acceptance"
			if:   _historicalCondition + " && !inputs.performance"
			env:  _historicalEnvironment
			if _platform == "linux" {
				run: (linuxSecretService.session & {command: "mise exec --locked -- go run ./tools/release accept-native \(_historicalArguments)"}).run
			}
			if _platform != "linux" {
				run: "mise exec --locked -- go run ./tools/release accept-native \(_historicalArguments)"
			}
		},
		{
			name: "Measure historical release performance"
			if:   _historicalCondition + " && inputs.performance"
			env:  _historicalEnvironment
			if _platform == "linux" {
				run: (linuxSecretService.session & {command: _performanceCommand}).run
			}
			if _platform != "linux" {
				run: _performanceCommand
			}
		},
		if _platform == "windows" {
			name: "Remove official Windows client supply"
			if:   "always() && github.event_name == 'workflow_dispatch' && inputs.windows_clients && inputs.baseline_tag != ''"
			run: #"""
				foreach ($owned in @($env:AIGW_NATIVE_CLIENT_SUPPLY, $env:AIGW_HERMES_INSTALLER)) {
				  if ($owned -and (Test-Path -LiteralPath $owned)) { Remove-Item -LiteralPath $owned -Recurse -Force -ErrorAction Stop }
				  if ($owned -and (Test-Path -LiteralPath $owned)) { throw "Windows native supply remains after cleanup: $owned" }
				}
				"""#
		},
		#DependencyEvidenceGitHubStep & {_name: "dependency-evidence-\(_platform)"},
		#SourceEvidenceGitHubStep & {_name: "source-evidence-\(_platform)"},
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
	_platform:          #OperatingSystem
	_prepareMise:       string
	_install:           string
	_refreshLocks:      string
	_native:            string
	_prebuiltCondition: string
	_selectTools:       string
	_bootstrap:         string
	_cleanup:           string
	tags: [string, ...string]
	rules: [...{...}]
	if _platform == "windows" {
		_prebuiltCondition: "($env:AIGW_NATIVE_INPUT_PACKAGE -or $env:AIGW_CANDIDATE_ARTIFACTS -or $env:AIGW_CANDIDATE_TAG) -and $env:AIGW_FULL_NATIVE_QUALITY -ne 'true' -and $env:AIGW_REFRESH_LOCKS -ne 'true'"
		_selectTools:       "if (\(_prebuiltCondition)) { $env:MISE_ENABLE_TOOLS = '\(nativeArtifactToolchain[_platform].MISE_ENABLE_TOOLS)'; if ($env:AIGW_NATIVE_PERFORMANCE -eq 'true') { $env:MISE_ENABLE_TOOLS += ',\(strings.Join(toolchainTools.performance, ","))' } }; if ($env:AIGW_NATIVE_INPUT_PACKAGE -and (\(nativeWindowsClientSelection))) { $env:MISE_ENABLE_TOOLS += ',node,uv' }"
		_bootstrap:         "if (-not (\(_prebuiltCondition))) { \(commands.bootstrap) }"
		_prepareMise:       #"""
			$ErrorActionPreference = 'Stop'
			foreach ($name in @('CI_API_V4_URL', 'CI_PROJECT_ID', 'CI_PROJECT_DIR', 'CI_JOB_ID', 'CI_JOB_TOKEN', 'CI_SERVER_HOST')) {
			  if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name))) { throw "Missing GitLab job input: $name" }
			}
			$jobDirectory = \#(windowsMiseJobDirectory)
			if (Test-Path -LiteralPath $jobDirectory) { throw 'Mise job directory already exists.' }
			[void](New-Item -ItemType Directory -Path $jobDirectory -ErrorAction Stop)
			$identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
			& icacls.exe $jobDirectory /inheritance:r /grant:r "${identity}:(OI)(CI)F" | Out-Null
			if ($LASTEXITCODE -ne 0) { throw 'Cannot restrict Mise job directory ACL.' }
			foreach ($name in @('MISE_CONFIG_DIR', 'MISE_CACHE_DIR', 'MISE_STATE_DIR', 'MISE_DATA_DIR')) {
			  $path = Join-Path $jobDirectory $name
			  [Environment]::SetEnvironmentVariable($name, $path)
			  [void](New-Item -ItemType Directory -Path $path -ErrorAction Stop)
			}
			$env:MISE_TRUSTED_CONFIG_PATHS = $env:CI_PROJECT_DIR
			$mise = Join-Path $env:ProgramFiles 'mise\bin\mise.exe'
			if (-not (Test-Path -LiteralPath $mise -PathType Leaf)) { throw 'Runner-owned Mise is missing.' }
			if ((Get-FileHash -LiteralPath $mise -Algorithm SHA256 -ErrorAction Stop).Hash -ne '\#(miseWindowsArm64ExecutableSHA256)') { throw 'Runner-owned Mise digest differs from the admitted release.' }
			$reported = & $mise --version
			if ($LASTEXITCODE -ne 0) { throw 'Runner-owned Mise failed to start under the job identity.' }
			if ($reported -notmatch ('^' + [regex]::Escape('\#(miseVersion)') + '(\s|$)')) { throw 'Runner-owned Mise version differs from the admitted release.' }
			& whoami.exe /user
			if ($LASTEXITCODE -ne 0) { throw 'Runner identity could not be observed.' }
			$shim = Join-Path (Split-Path -Parent $mise) 'mise-shim.exe'
			if (-not (Test-Path -LiteralPath $shim -PathType Leaf)) { throw 'Runner-owned Mise shim is missing.' }
			try { $shimHash = (Get-FileHash -LiteralPath $shim -Algorithm SHA256 -ErrorAction Stop).Hash }
			catch { & icacls.exe $shim; throw "Runner-owned Mise shim cannot be read (error=$($_.FullyQualifiedErrorId), hresult=$($_.Exception.HResult))." }
			if ($shimHash -ne '\#(miseWindowsArm64ShimSHA256)') { throw 'Runner-owned Mise shim digest differs from the admitted release.' }
			$env:PATH = (Split-Path -Parent $mise) + [IO.Path]::PathSeparator + $env:PATH
			if ($env:AIGW_TOOL_SOURCE -and $env:AIGW_TOOL_SOURCE -notin @('upstream', 'peer')) { throw 'AIGW_TOOL_SOURCE must be upstream or peer.' }
			if ($env:AIGW_TOOL_SOURCE -eq 'peer') {
			$netrc = Join-Path $jobDirectory '_netrc'
			[IO.File]::WriteAllText($netrc, "machine $env:CI_SERVER_HOST login gitlab-ci-token password $env:CI_JOB_TOKEN`n", [Text.UTF8Encoding]::new($false))
			& icacls.exe $netrc /inheritance:r /grant:r "${identity}:R" | Out-Null
			if ($LASTEXITCODE -ne 0) { throw 'Cannot restrict Mise mirror credential ACL.' }
			$lockDigest = (Get-FileHash -LiteralPath (Join-Path $env:CI_PROJECT_DIR 'mise.lock') -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant()
			$mirrorBase = "$env:CI_API_V4_URL/projects/$env:CI_PROJECT_ID/\#(miseMirror.resource)$lockDigest/"
			$replacements = [ordered]@{}
			$replacements['\#(miseMirror.metadataPattern)'] = "${mirrorBase}" + '\#(miseMirror.metadataResource)'
			$replacements['https://github.com/'] = $mirrorBase
			$replacements['https://api.github.com/'] = $mirrorBase
			$env:MISE_NETRC_FILE = $netrc
			$env:MISE_NETRC = 'true'
			$env:MISE_URL_REPLACEMENTS = $replacements | ConvertTo-Json -Compress
			}
			\#(_selectTools)
			"""#
		_install:           "cmd /c \"set GODEBUG=\(installationEnvironment.GODEBUG)&&mise install --locked\""
		_refreshLocks:      "if ($env:AIGW_REFRESH_LOCKS -eq 'true') { \(commands.resolveLocks) }"
		_native:            #"""
			$ErrorActionPreference = 'Stop'
			$PSNativeCommandUseErrorActionPreference = $true
			if (($env:AIGW_NATIVE_DIAGNOSTIC_CLIENT -or $env:AIGW_NATIVE_PERFORMANCE -eq 'true' -or $env:AIGW_NATIVE_PERFORMANCE_ATTRIBUTION -eq 'true') -and $env:CI_PIPELINE_SOURCE -notin @('web', 'api')) { throw 'Native diagnostics and performance require a manual pipeline' }
			\#(nativeWindowsClientSupply)
			$acceptance = @('--peer', 'gitlab', '--repository', $env:CI_PROJECT_URL)
			if ($env:AIGW_BASELINE_TAG) { $acceptance += @('--baseline-tag', $env:AIGW_BASELINE_TAG) }
			if ($env:AIGW_BASELINE_ARTIFACTS) { $acceptance += @('--baseline-artifacts', $env:AIGW_BASELINE_ARTIFACTS) }
			if ($env:AIGW_CANDIDATE_TAG) { $acceptance += @('--tag', $env:AIGW_CANDIDATE_TAG) }
			if ($env:AIGW_CANDIDATE_ARTIFACTS) { $acceptance += @('--artifacts', $env:AIGW_CANDIDATE_ARTIFACTS, '--candidate') }
			if ($env:AIGW_CANDIDATE_SOURCE) { $acceptance += @('--candidate-source', $env:AIGW_CANDIDATE_SOURCE) }
			if ($env:AIGW_NATIVE_INPUT_PACKAGE) {
			  if (\#(nativeWindowsClientSelection)) {
			    $acceptance += @('--input-archive', $archive)
			  } else {
			    $acceptance += @('--input-package', $env:AIGW_NATIVE_INPUT_PACKAGE)
			  }
			  $acceptance += @('--input-sha256', $env:AIGW_NATIVE_INPUT_SHA256, '--candidate')
			}
			if ($env:AIGW_NATIVE_PERFORMANCE -eq 'true') { $acceptance += @('--performance', (Join-Path (Get-Location).ProviderPath 'build/verification/performance')) }
			if ($env:AIGW_NATIVE_PERFORMANCE_ATTRIBUTION -eq 'true') { $acceptance += @('--performance-attribution') }
			if ($env:AIGW_NATIVE_DIAGNOSTIC_CLIENT) {
			  $acceptance += @('--diagnostic-client', $env:AIGW_NATIVE_DIAGNOSTIC_CLIENT)
			} elseif ($env:AIGW_NATIVE_CLIENTS -eq 'true') {
			  if (-not $env:AIGW_BASELINE_TAG) { throw 'Real-client succession requires AIGW_BASELINE_TAG.' }
			  $acceptance += '--clients'
			}
			if (-not $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE) { $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE = $env:AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS }
			if (-not $env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE) { $env:AIGW_RELEASE_ALLOWED_SIGNERS_FILE = $env:AIGW_RELEASE_ALLOWED_SIGNERS }
			\#(commands.native[_platform]) --full-quality="$($env:AIGW_FULL_NATIVE_QUALITY -eq 'true')" -- @acceptance
			"""#
		_cleanup:           #"""
			$ErrorActionPreference = 'Stop'
			$jobDirectory = \#(windowsMiseJobDirectory)
			if (Test-Path -LiteralPath $jobDirectory) {
			  $emptyDirectory = Join-Path (Split-Path -Parent $jobDirectory) "aigw-ci-mise-empty-$env:CI_JOB_ID"
			  if (Test-Path -LiteralPath $emptyDirectory) { throw 'Mise cleanup mirror source already exists.' }
			  [IO.Directory]::CreateDirectory($emptyDirectory) | Out-Null
			  try {
			    & robocopy.exe $emptyDirectory $jobDirectory /MIR /R:1 /W:1 /NFL /NDL /NJH /NJS /NP | Out-Null
			    $mirrorExit = $LASTEXITCODE
			    if ($mirrorExit -ge 8) { throw "Mise cleanup mirror failed (robocopy exit $mirrorExit)." }
			    [IO.Directory]::Delete($jobDirectory)
			  } catch {
			    $cause = $_.Exception
			    Write-Host "Mise cleanup failure type=$($cause.GetType().FullName) HRESULT=0x$($cause.HResult.ToString('X8'))"
			    throw
			  } finally {
			    if (Test-Path -LiteralPath $emptyDirectory) { [IO.Directory]::Delete($emptyDirectory) }
			  }
			}
			if (Test-Path -LiteralPath $jobDirectory) {
			  throw 'Mise job directory remains after cleanup.'
			}
			Write-Host 'Mise job-owned supply state retired.'
			"""#
	}
	if _platform != "windows" {
		_cleanup:           miseMirror.unixCleanup
		_prebuiltCondition: "[ -n \"${AIGW_NATIVE_INPUT_PACKAGE:-}${AIGW_CANDIDATE_ARTIFACTS:-}${AIGW_CANDIDATE_TAG:-}\" ] && [ \"${AIGW_FULL_NATIVE_QUALITY:-false}\" != true ] && [ \"${AIGW_REFRESH_LOCKS:-false}\" != true ]"
		_selectTools:       "if \(_prebuiltCondition); then export MISE_ENABLE_TOOLS='\(nativeArtifactToolchain[_platform].MISE_ENABLE_TOOLS)'; if [ \"${AIGW_NATIVE_PERFORMANCE:-false}\" = true ]; then export MISE_ENABLE_TOOLS=\"$MISE_ENABLE_TOOLS,\(strings.Join(toolchainTools.performance, ","))\"; fi; fi"
		_bootstrap:         "if ! { \(_prebuiltCondition); }; then \(commands.bootstrap); fi"
		_install:           commands.install
		_refreshLocks:      "if [ \"${AIGW_REFRESH_LOCKS:-false}\" = true ]; then \(commands.resolveLocks); fi"
		_nativeCommand:     "\(commands.native[_platform]) --full-quality=\"${AIGW_FULL_NATIVE_QUALITY:-false}\" -- \"$@\""
		_nativeExecution:   string
		if _platform != "linux" {
			_nativeExecution: _nativeCommand
		}
		if _platform == "linux" {
			_nativeExecution: #"""
				if [ -n "${AIGW_NATIVE_INPUT_PACKAGE:-}${AIGW_CANDIDATE_ARTIFACTS:-}${AIGW_CANDIDATE_TAG:-}${AIGW_BASELINE_TAG:-}${AIGW_BASELINE_ARTIFACTS:-}" ]; then
				\#((linuxSecretService.session & {command: _nativeCommand}).run)
				else
				  \#(_nativeCommand)
				fi
				"""#
		}
		_native: #"""
			if { [ -n "${AIGW_NATIVE_DIAGNOSTIC_CLIENT:-}" ] || [ "${AIGW_NATIVE_PERFORMANCE:-false}" = true ] || [ "${AIGW_NATIVE_PERFORMANCE_ATTRIBUTION:-false}" = true ]; } && [ "${CI_PIPELINE_SOURCE:-}" != web ] && [ "${CI_PIPELINE_SOURCE:-}" != api ]; then
			  printf '%s\n' 'Native diagnostics and performance require a manual pipeline' >&2
			  exit 1
			fi
			set -eu
			set -- --peer gitlab --repository "$CI_PROJECT_URL"
			if [ -n "${AIGW_BASELINE_TAG:-}" ]; then set -- "$@" --baseline-tag "$AIGW_BASELINE_TAG"; fi
			if [ -n "${AIGW_BASELINE_ARTIFACTS:-}" ]; then set -- "$@" --baseline-artifacts "$AIGW_BASELINE_ARTIFACTS"; fi
			if [ -n "${AIGW_CANDIDATE_TAG:-}" ]; then set -- "$@" --tag "$AIGW_CANDIDATE_TAG"; fi
			if [ -n "${AIGW_CANDIDATE_ARTIFACTS:-}" ]; then set -- "$@" --artifacts "$AIGW_CANDIDATE_ARTIFACTS" --candidate; fi
			if [ -n "${AIGW_CANDIDATE_SOURCE:-}" ]; then set -- "$@" --candidate-source "$AIGW_CANDIDATE_SOURCE"; fi
			if [ -n "${AIGW_NATIVE_INPUT_PACKAGE:-}" ]; then set -- "$@" --input-package "$AIGW_NATIVE_INPUT_PACKAGE" --input-sha256 "$AIGW_NATIVE_INPUT_SHA256" --candidate; fi
			if [ "${AIGW_NATIVE_PERFORMANCE:-false}" = true ]; then set -- "$@" --performance "$(pwd -P)/build/verification/performance"; fi
			if [ "${AIGW_NATIVE_PERFORMANCE_ATTRIBUTION:-false}" = true ]; then set -- "$@" --performance-attribution; fi
			if [ -n "${AIGW_NATIVE_DIAGNOSTIC_CLIENT:-}" ]; then
			  set -- "$@" --diagnostic-client "$AIGW_NATIVE_DIAGNOSTIC_CLIENT"
			elif [ "${AIGW_NATIVE_CLIENTS:-false}" = true ]; then
			  : "${AIGW_BASELINE_TAG:?Real-client succession requires AIGW_BASELINE_TAG}"
			  set -- "$@" --clients
			fi
			export AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE="${AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE:-${AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS:-}}"
			export AIGW_RELEASE_ALLOWED_SIGNERS_FILE="${AIGW_RELEASE_ALLOWED_SIGNERS_FILE:-${AIGW_RELEASE_ALLOWED_SIGNERS:-}}"
			\#(_nativeExecution)
			"""#
	}
	"after_script": [_cleanup]
	stage: graph["native-\(_platform)"].stage
	variables: nativeToolchain[_platform] & {
		GLAB_NO_PROMPT: "1"
		if _platform == "windows" {
			AIGW_VERIFY_SYSTEM_KEYRING: "1"
		}
	}
	artifacts: {
		when: "always"
		paths: ["./{mise.lock,.mise/locks,build/verification}"]
	}
	if _platform == "linux" {
		interruptible: true
		extends: [".linux-toolchain"]
		variables: CGO_ENABLED: "1"
		"before_script": [
			linuxToolchain.prepare,
			"if \(_prebuiltCondition); then export CGO_ENABLED=0; else export CGO_ENABLED=1; \(linuxToolchain.compiler); fi",
			"DEBIAN_FRONTEND=noninteractive \(linuxApt.install) \(linuxSecretService.packages)",
			miseMirror.unixPrepare, _selectTools, commands.install,
		]
		script: [_bootstrap, _refreshLocks, _native, _cleanup]
	}
	if _platform != "linux" {
		if _platform == "windows" {
			script: [_prepareMise, _install, _bootstrap, _refreshLocks, _native, _cleanup]
		}
		if _platform != "windows" {
			script: [miseMirror.unixPrepare, _selectTools, _install, _bootstrap, _refreshLocks, _native, _cleanup]
		}
	}
}

_nativeManualCondition: {
	for platform in productEvidence.native {
		(platform): "(\(gitlabVerificationCondition.manual)) && ($AIGW_NATIVE_PLATFORM == null || $AIGW_NATIVE_PLATFORM == \"\" || $AIGW_NATIVE_PLATFORM == \"all\" || $AIGW_NATIVE_PLATFORM == \"\(platform)\")"
	}
}

_gitlabControlJob: {
	_commands: [...string]
	tags: nativeEvidence[gitlabControlPlatform].gitlab.tags
	if gitlabControlPlatform == "linux" {
		extends: [".linux-toolchain"]
		script: list.Concat([_commands, [miseMirror.unixCleanup]])
	}
	if gitlabControlPlatform != "linux" {
		script: list.Concat([[miseMirror.unixPrepare, commands.install], _commands, [miseMirror.unixCleanup]])
		"after_script": [miseMirror.unixCleanup]
	}
}

gitlab: {
	variables: gitEnvironment & {
		GIT_DEPTH:                  "0"
		FF_GIT_URLS_WITHOUT_TOKENS: "true"
		GOPROXY:                    "https://goproxy.cn|https://proxy.golang.org|direct"
		AIGW_TOOL_SOURCE:           "upstream"
	}
	workflow: {
		auto_cancel: on_new_commit: "conservative"
		rules: gitlabPipelineRules
	}
	stages: ["verify", "release"]
	".linux-toolchain": {
		_dataDirectory: "build/runtime/tool-cache/.mise/$AIGW_TOOL_SOURCE"
		_cacheDirectories: ["installs", "cache"]
		image: miseImage
		variables: {
			AIGW_MISE_DATA_ROOT: "$CI_PROJECT_DIR/\(_dataDirectory)"
			MISE_DATA_DIR:       "$CI_PROJECT_DIR/\(_dataDirectory)"
			MISE_CACHE_DIR:      "$CI_PROJECT_DIR/\(_dataDirectory)/cache"
		}
		cache: {
			key: {
				files: ["mise.toml", "mise.lock"]
				prefix: "mise-linux-\(strings.Split(miseImage, "@sha256:")[1])-\(strings.Join(strings.Split(_dataDirectory, "/"), "-"))-\(strings.Join(_cacheDirectories, "-"))-$CI_RUNNER_ID-$CI_JOB_NAME"
			}
			paths: [for directory in _cacheDirectories {"\(_dataDirectory)/\(directory)/"}]
			policy: "pull-push"
			when:   "always"
		}
		"before_script": [linuxToolchain.prepare, miseMirror.unixPrepare, commands.install]
		"after_script": [miseMirror.unixCleanup]
	}
	quality: {
		interruptible: true
		extends: [".linux-toolchain"]
		tags: nativeEvidence.linux.gitlab.tags
		script: [
			commands.bootstrap,
			"export AIGW_RELEASE_ALLOWED_SIGNERS_FILE=\"$AIGW_RELEASE_ALLOWED_SIGNERS\"",
			commands.quality,
			miseMirror.unixCleanup,
		]
		artifacts: {when: "always", paths: [dependencyEvidencePath, workflowEvidencePath, "build/verification/coverage"]}

		stage: graph.quality.stage
		variables: qualityToolchain & {CGO_ENABLED: "0"}
		rules: [
			{if: gitlabVerificationCondition.tag, variables: AIGW_COMMIT_BASE: "$CI_COMMIT_SHA^"},
			{
				if: gitlabVerificationCondition.review
				variables: AIGW_COMMIT_BASE: "$CI_MERGE_REQUEST_DIFF_BASE_SHA"
			},
			{
				if: gitlabVerificationCondition.protectedPush
				variables: AIGW_COMMIT_BASE: "$CI_COMMIT_BEFORE_SHA"
			},
			{if: gitlabVerificationCondition.manualSource, variables: AIGW_COMMIT_BASE: "$CI_COMMIT_SHA^"},
			{when: "never"},
		]
	}
	"accepted-ref-parity": _gitlabControlJob & {
		_commands: [commands.acceptedRefParity.gitlab]
		stage:     graph["accepted-ref-parity"].stage
		variables: goToolchain
		rules: [
			{if: "$CI_PIPELINE_SOURCE == \"push\" && $CI_COMMIT_BRANCH == \"\(lifecycle.releaseBranch)\""},
			{when: "never"},
		]
	}
	for platform in productEvidence.native {
		if platform == "linux" {
			"native-linux": #NativeGitLabJob & {
				_platform: platform
				tags:      nativeEvidence.linux.gitlab.tags
				rules: [
					{if: gitlabVerificationCondition.tag},
					{if: gitlabVerificationCondition.review},
					{if: gitlabVerificationCondition.protectedPush},
					{if: _nativeManualCondition[platform]},
					{when: "never"},
				]
			}
		}
		if platform != "linux" {
			"native-\(platform)": #NativeGitLabJob & {
				_platform: platform
				tags: [nativeEvidence[platform].gitlab.protectedTag]
				rules: [
					{if: gitlabVerificationCondition.tag},
					{if: gitlabVerificationCondition.protectedPush},
					{if: "(\(_nativeManualCondition[platform])) && $CI_COMMIT_REF_PROTECTED == \"true\""},
					{when: "never"},
				]
			}
			"native-\(platform)-review": #NativeGitLabJob & {
				_platform: platform
				tags: [nativeEvidence[platform].gitlab.reviewTag]
				rules: [
					{if: gitlabVerificationCondition.review},
					{if: "(\(_nativeManualCondition[platform])) && $CI_COMMIT_REF_PROTECTED == \"false\""},
					{when: "never"},
				]
			}
		}
	}
	"linux-secret-service": {
		interruptible: true
		extends: [".linux-toolchain"]
		stage: graph["linux-secret-service"].stage
		tags:  nativeEvidence.linux.gitlab.tags
		variables: {
			MISE_ENABLE_TOOLS: strings.Join(toolchainTools.secretService, ",")
			CGO_ENABLED:       "0"
		}
		rules: gitlab.quality.rules
		script: [linuxSecretService.gitlab, miseMirror.unixCleanup]
	}
	"release-version": _gitlabControlJob & {
		_commands: [commands.version]
		stage:     graph["release-version"].stage
		variables: goToolchain
		rules: [
			{if: "$CI_COMMIT_TAG"},
			{when: "never"},
		]
	}
	"release-assets": {
		extends: [".linux-toolchain"]
		script: [
			#"mkdir dist"#,
			#"mise exec --locked -- glab release download "$CI_COMMIT_TAG" --repo "$CI_PROJECT_URL" --asset-name 'aigw_*' --asset-name 'checksums.txt*' --dir dist"#,
			commands.artifacts,
			miseMirror.unixCleanup,
		]
		stage:     graph["release-assets"].stage
		tags:      nativeEvidence.linux.gitlab.tags
		variables: #ReleaseTrustFiles
		rules: [
			{
				if: "$CI_COMMIT_TAG && ($CI_PIPELINE_SOURCE == \"api\" || $CI_PIPELINE_SOURCE == \"web\") && $CI_COMMIT_REF_PROTECTED == \"true\""
			},
			{when: "never"},
		]
		needs: [for dependency in graph["release-assets"].needs {{job: dependency}}]
	}

}

githubVerify: {
	name: "Verify"
	defaults: run: shell: "bash"
	env: gitEnvironment
	"on": {
		push: {branches: [lifecycle.acceptedBranch, lifecycle.releaseBranch], tags: ["v*"]}
		"pull_request": branches: [lifecycle.acceptedBranch, lifecycle.releaseBranch]
		"workflow_dispatch": inputs: {
			tool_source: toolSourceInput
			native_platform: {
				description: "Native platform to qualify; partial runs do not establish full release readiness"
				required:    false
				type:        "choice"
				default:     "all"
				options: ["all", for platform in productEvidence.native {platform}]
			}
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
			diagnostic_client: {
				description: "Diagnose one supplied native client with baseline/candidate inputs; not full acceptance"
				required:    false
				type:        "string"
				default:     ""
			}
			macos_keychain: {
				description: "With baseline_tag, qualify only the published predecessor Keychain journey on disposable macOS"
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
			performance_attribution: {
				description: "With performance=true, diagnose components without qualifying performance budgets"
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
				description: "Optional exclusive base; defaults to the selected commit's parent for manual diagnostics"
				required:    false
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
			input_release: {
				description: "Candidate-bound native-inputs-<SHA> transport release; requires baseline_tag, input_sha256 and candidate_source"
				required:    false
				type:        "string"
			}
			input_sha256: {
				description: "Exact SHA256 of the selected public-inputs.tar; verified before extraction"
				required:    false
				type:        "string"
			}
			candidate_source: {
				description: "Exact signed candidate source SHA for the selected native input release"
				required:    false
				type:        "string"
			}
		}
	}
	permissions: contents: "read"
	concurrency: {
		group:                "verify-${{ github.workflow }}-${{ github.ref }}-${{ github.event_name == 'workflow_dispatch' && github.run_id || 'automatic' }}"
		"cancel-in-progress": true
	}
	jobs: {
		"accepted-ref-parity": {
			name:              graph["accepted-ref-parity"].name
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
			name:              "${{ github.event_name == 'workflow_dispatch' && 'Manual \(graph.quality.name)' || '\(graph.quality.name)' }}"
			"runs-on":         nativeEvidence.linux.github.runner
			"timeout-minutes": 25
			if:                githubSourceVerificationCondition
			env:               qualityToolchain
			steps: [
				#SourceCheckout,
				#Toolchain,
				{
					name: "Prepare native memory measurement"
					run:  "set -eu\nsudo -n \(linuxApt.update)\nsudo -n DEBIAN_FRONTEND=noninteractive \(linuxApt.install) time"
				},
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
						CGO_ENABLED:                       "0"
						AIGW_COMMIT_BASE:                  githubCommitBase
						AIGW_RELEASE_AUTHOR_EMAIL:         "${{ vars.AIGW_RELEASE_AUTHOR_EMAIL }}"
						AIGW_RELEASE_ALLOWED_SIGNERS_FILE: "${{ env.AIGW_RELEASE_ALLOWED_SIGNERS_FILE }}"
						AIGW_CHANGELOG_RELEASE_TAG:        "${{ github.ref_type == 'tag' && github.ref_name || '' }}"
					}
					run: commands.quality
				},
				#DependencyEvidenceGitHubStep & {_name: "dependency-evidence-quality"},
				#SourceEvidenceGitHubStep & {_name: "source-evidence-quality"},

			]
		}
		"linux-secret-service": {
			name:              "${{ github.event_name == 'workflow_dispatch' && 'Manual \(graph["linux-secret-service"].name)' || '\(graph["linux-secret-service"].name)' }}"
			"runs-on":         nativeEvidence.linux.github.runner
			"timeout-minutes": 25
			if:                githubSourceVerificationCondition
			env: {
				MISE_ENABLE_TOOLS: strings.Join(toolchainTools.secretService, ",")
				CGO_ENABLED:       "0"
			}
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Qualify Linux Secret Service", run: linuxSecretService.github},
			]
		}
		"release-version": {
			name:              "${{ github.event_name == 'workflow_dispatch' && 'Manual \(graph["release-version"].name)' || '\(graph["release-version"].name)' }}"
			"runs-on":         nativeEvidence.linux.github.runner
			"timeout-minutes": 5
			if:                "github.ref_type == 'tag'"
			env:               goToolchain
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Validate tag version", run: commands.version},
			]
		}
		for platform in productEvidence.native {
			"native-\(platform)": #NativeGitHubJob & {_platform: platform}
		}
	}
}

githubRelease: {
	name: "Release"
	env:  gitEnvironment
	"on": {
		"workflow_dispatch": inputs: {
			tool_source: toolSourceInput
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
	permissions: {
		contents: "read"
		actions:  "read"
	}
	concurrency: {
		group:                "release-${{ github.repository }}-${{ inputs.tag }}-${{ inputs.runner }}"
		"cancel-in-progress": false
	}
	jobs: {
		"release-assets": {
			name:              graph["release-assets"].name
			"runs-on":         "${{ inputs.runner }}"
			"timeout-minutes": 25
			defaults: run: shell: "pwsh"
			env: {
				MISE_ENABLE_TOOLS:            "${{ inputs.native_lifecycle && (startsWith(inputs.runner, 'macos-') && 'go,gh,node,github:goreleaser/goreleaser,github:indygreg/apple-platform-rs' || 'go,gh,node,github:goreleaser/goreleaser') || 'go,gh' }}"
				GH_TOKEN:                     "${{ github.token }}"
				CI_COMMIT_TAG:                "${{ inputs.tag }}"
				AIGW_RELEASE_ARTIFACT_SIGNER: "${{ vars.AIGW_RELEASE_ARTIFACT_SIGNER }}"
			}
			steps: [
				#SourceCheckout,
				#Toolchain,
				{name: "Validate release version", run: commands.version},
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
					name: "Verify this peer's exact tag pipeline"
					run:  """
						$tagCommit = git rev-parse --verify "$($env:CI_COMMIT_TAG)^{commit}"
						if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($tagCommit)) { throw 'Cannot resolve selected release tag' }
						mise exec --locked -- go run ./tools/ci release-evidence --repository "$env:GITHUB_REPOSITORY" --workflow verify.yml --tag "$env:CI_COMMIT_TAG" --sha "$tagCommit" \(githubTagEvidenceJobs)
						"""
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
						AIGW_VERIFY_SYSTEM_KEYRING:        "${{ runner.os == 'Windows' && '1' || '0' }}"
						AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE: "ephemeral-host"
					}
					run: "mise exec --locked -- go run ./tools/release accept-native --artifacts dist"
				},
			]
		}
	}
}
