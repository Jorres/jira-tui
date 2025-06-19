# Jira TUI

**A feature-rich terminal interface for Jira that lets you (almost) never use the browser client.**

> [!NOTE]
> This tool is feature-complete but documentation is still growing. The README will be expanded into a comprehensive guide.

![Demo](./demo.gif)

> [!WARNING]
> Currently documented for **Jira Cloud** only. On-premise Jira documentation coming soon.

## Features

- **Create & edit issues** with full Jira features (mentions, attachments)
- **Create & edit comments** with rich text support
- **Mention colleagues** using `@email` syntax
- **Assign issues** to team members
- **Link issues to epics**
- **Search** by issue name or key
- **Dual interface**: Interactive TUI or traditional CLI (full docs for CLI are coming soon, for now `jira --help`)

## Quick Start

1. **Download** a pre-built binary from [releases](https://github.com/Jorres/jira-tui/releases)
2. **Get an API token from your Jira installation**: [Atlassian guide](https://id.atlassian.com/manage-profile/security/api-tokens) and export it:
   ```bash
   export JIRA_API_TOKEN="your-token-here"
   ```
3. **Initialize**: Run `jira init`, select `Cloud`, and provide your Jira details
4. **Launch**: Run `jira ui` and press `?` for help

## Customization

Create multiple tabs with custom filters and views. Each tab can fetch different issues with its own predicates. Add a `ui` section to your config file:

### Multi-Tab Example

Here's a configuration with two tabs - one filtered (EXAMPLE_DEV) and one unfiltered (EXAMPLE_OPS):

```yaml
ui:
  list:
    tabs:
      - name: "tab 1"
        project: "EXAMPLE_DEV"
        assignee: "jorres@example.com"
        status: ["~Done", "~Closed"]
        columns: ["KEY", "TYPE", "PARENT", "SUMMARY", "STATUS", "ASSIGNEE", "REPORTER", "CREATED", "PRIORITY"]
        orderBy: "updated"

        # ... and there are all possible filters with example values:
        # priority: "Highest"
        # watching: true
        # issueType: "Epic"
        # parent: YOURISSUE-931
        # created: "-20d"
        # createdAfter: "2025-04-27"    # Issues created after 2025-04-27
        # createdBefore: "2025-04-29"   # Issues created before 2025-04-29
        # reporter: "Egor Tarasov"
        # orderBy: "priority"           # Options: created, updated, priority, status, etc.
        # reverse: true                 # Sort descending instead of ascending
        # from: 0                       # Start from first result (pagination)
        # limit: 10                     # Return max 50 results

        # ... this is the special filter, you can just write JQL and it will override all the other filters
        # jql: 'project = "YOURISSUE" AND assignee = currentUser()'
      - name: "tab 2"
        project: "EXAMPLE_OPS"
        assignee: "jorres@nebius.com"
        status: []
        columns: ["KEY", "TYPE", "PARENT", "SUMMARY", "STATUS", "ASSIGNEE", "REPORTER", "CREATED", "PRIORITY"]
        ...
```

### Theming

Customize colors using the `theme` section. Colors can be hex codes (`#AABBCC`) or [8-bit ANSI values](https://en.wikipedia.org/wiki/ANSI_escape_code#8-bit) (0-255):

- **accent**: Highlight color (default violet elements)
- **pale**: Border and secondary elements color

**Default theme:**

```yaml
ui:
  theme:
    accent: "62" # Purple highlight
    pale: "240" # Gray borders
```

**Custom theme example:**

```yaml
ui:
  theme:
    accent: "#859900" # Solarized green
    pale: "240"
```
