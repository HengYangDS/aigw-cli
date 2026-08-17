# Design

The existing selected Go package and declared tool closure, release pipeline,
and Forge projection commands already own this transition. Update reports for
modules outside that compiled closure are discovery noise rather than selected
product dependencies. The change updates only their declared inputs and
release identity; it adds no package, adapter, wrapper, compatibility path, or
new authority.

The local work lane first refreshes the module graph and proves source and
coverage. After exact-HEAD proof, normal lifecycle commands land the same tree.
GitLab and GitHub then construct provider-native signed histories from that
tree and publish independently. Installation consumes one signed release asset,
not a source checkout or peer Forge.
