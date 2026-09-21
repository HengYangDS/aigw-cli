# AIGW Architecture Edition

This optional package owns AIGW's curated Architecture Publisher projection.
It consumes one caller-selected Source Bundle and produces a Claim Model plus
an audience-specific Edition. It does not inspect Git, execute AIGW, mutate
configuration, read credentials, or add an AIGW command.

The source repository remains authoritative. The package binds declared
semantic entities and relations to exact selected source bytes without
inferring meaning from Go syntax or source-code markers.
