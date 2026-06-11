Sync the GitHub intake App configuration from the configured identity resolver.

Examples:
  gc github sync-app
  gc github sync-app --identity mayor
  gc github sync-app --json

By default the command reads `GITHUB_INTAKE_APP_IDENTITY` from the city
workspace environment, resolves it through `GITHUB_INTAKE_IDENTITY_RESOLVER`,
and writes the redacted App metadata plus secret material into the pack state.
Secret values are not printed.
