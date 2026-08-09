# Hosted verification forward fix

Close the two exact hosted-verification gaps: materialize GitLab trust input as
a file and remove a host-specific POSIX-mode precondition from portable
installation. Release admission remains the integrity boundary; installation
accepts one regular-file contract on macOS, Linux, and Windows.
