## Jira TUI

Jira TUI is a feature rich TUI that allows to (almost) never use the Jira browser client. What you can look forward to?

![Demo](./demo.mp4)

> [!WARNING]
> This is an early version for early adopters. README will be eventually expanded to a short wiki.

- creating editing issues with all supported jira features (mentioning people, adding attachments)
- creating \ editing comments
- pinging your colleagues through @<email> syntax
- assigning people to issues
- assigning issues to epics
- searching by issue name \ issue key
- ... all that in interactive TUI view, OR in regular CLI fasion (`jira --help`)

## Quick getting started

1. [Get a Jira API token](https://id.atlassian.com/manage-profile/security/api-tokens) and export it to your shell as
   a `JIRA_API_TOKEN` variable.
2. Run `jira init`, select installation type as `Cloud`, and provide required details to generate a config file required
   for the tool.
3. Run `jira ui` and enjoy. Press '?' for help.

## Customizing the view

You can create multiple tabs as in the preview, and each tabs can fetch its own issues and filter them with its own predicates. To do that, add a `ui` top-level field to
the config.

Here is a simple example with two tabs, looking into EXAMPLE_DEV and EXAMPLE_OPS projects. EXAMPLE_DEV applies some filtering, EXAMPLE_OPS does not.

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

5. Optionally, you can recolor the interface by adding `theme` key with `accent` and `pale` colors. Accent color is everything that's violet,
   pale color mainly stands for pale gray borders. Colors will be accepted either as `#AABBCC` or from 0 to 255, as per [8 bits ANSI colors](https://en.wikipedia.org/wiki/ANSI_escape_code#8-bit) Try a couple:

   This is how to set a default theme:

   ```yaml
   ui:
     theme:
       accent: "62"
       pale: "240"
   ```

   And this is how to override:

   ```yaml
   ui:
     theme:
       accent: "#859900"
       pale: "240"
   ```
