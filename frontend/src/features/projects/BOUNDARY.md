# Projects feature boundary

Owns the slim project rail (FH-150): All projects plus every project as a
key badge with its Needs you count and needs-repair mark, its name, key and
counts (tickets, in progress, blocked, needs repair, needs you) in the
badge's accessible name and tooltip, ordered as the API returns them (most
recent activity first), and selection of the current scope (PRJ-6). A
project's own image covers its badge (FH-151).

Owns the page header's project badge as a control, `ProjectImageMenu`:
choose, replace or remove the project's image, with the server's refusal
shown in the menu. The shell gives it the API calls and the toast.

Does not own loading the project list, the image endpoints (`api/files`),
routing (`app/`) or the card and board views.
