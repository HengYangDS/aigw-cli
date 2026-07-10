package catalog

import "gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"

const teamProfiles = `version = 1
recommended_default = "dmx"

[profiles.dmx]
label = "DMXAPI"

[profiles.dmx.endpoints]
openai_responses = "https://www.dmxapi.cn/v1"
anthropic = "https://www.dmxapi.cn"
`

func Team() (manifest.Manifest, error) { return manifest.Parse([]byte(teamProfiles)) }
