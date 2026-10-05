package migrate

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

// Options configures a migration.
type Options struct {
	// Keys overrides project keys by project name (--key alpha=AL).
	Keys map[string]string
	Now  func() time.Time
}

// Ticket is one v1 ticket file and where it goes.
type Ticket struct {
	OldPath, OldSlug string
	ID, Folder       string
	Status           board.Column
	Archived, Broken bool
	// Review and Files are the old review file and attachments folder, if any.
	Review, Files string
}

// Target is the new ticket folder, relative to the root.
func (t Ticket) Target(project string) string {
	if t.Archived {
		return path.Join(project, ".archive", "tickets", t.Folder)
	}
	return path.Join(project, "tickets", t.Folder)
}

// Project is one v1 project's migration.
type Project struct {
	Name, Key, KeySource string
	NextID               int
	Tickets              []Ticket
	// Workstreams are workstream files rewritten in place.
	Workstreams []string
	// Moves are the v1 folders moved into the backup after conversion.
	Moves []string
	// ProjectFile reports whether project.yaml exists.
	ProjectFile bool
}

// Plan is a complete, not yet applied migration.
type Plan struct {
	Root     string
	Backup   string
	Projects []Project
	Warnings []string
}

var v1Status = map[string]board.Column{
	"todo": board.Backlog, "in-progress": board.InProgress, "ready-to-review": board.Review, "done": board.Done,
}

var typePrefix = regexp.MustCompile(`^[a-z]+--`)

// Prepare reads a v1 root and plans its migration. It writes nothing; a root
// with no v1 projects yields an empty plan.
func Prepare(s *store.Store, options Options) (Plan, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	plan := Plan{Root: s.Path(), Backup: ".flashheart/backup/v1-" + options.Now().UTC().Format("20060102T150405Z")}
	names, err := s.V1Projects()
	if err != nil {
		return Plan{}, err
	}
	if len(names) == 0 {
		return plan, nil // already v2: nothing to do, whatever keys were given
	}
	for name, key := range options.Keys {
		if !slices.Contains(names, name) {
			return Plan{}, fmt.Errorf("--key %s=%s: %s is not a v1 project in this root", name, key, name)
		}
		if !board.ValidKey(key) {
			return Plan{}, fmt.Errorf("--key %s=%q: a key is 2–10 uppercase letters or digits starting with a letter", name, key)
		}
	}

	owners := map[string]string{}
	if existing, err := s.Projects(); err == nil {
		for _, name := range existing {
			if project, err := s.ReadProject(name); err == nil {
				owners[project.Key] = name
			}
		}
	}
	slugs := map[string]map[string]string{} // project → v1 slug → id
	folders := map[string]map[string]string{}
	for _, name := range names {
		project, err := prepareProject(s, name, options)
		if err != nil {
			return Plan{}, err
		}
		if owner, taken := owners[project.Key]; taken {
			return Plan{}, fmt.Errorf("key %s for %s is already used by %s; choose another with --key %s=<KEY>", project.Key, name, owner, name)
		}
		owners[project.Key] = name
		slugs[name], folders[name] = map[string]string{}, map[string]string{}
		for _, ticket := range project.Tickets {
			slugs[name][ticket.OldSlug] = ticket.ID
			folders[name][ticket.OldSlug] = ticket.Folder
		}
		plan.Projects = append(plan.Projects, project)
	}
	for _, project := range plan.Projects {
		for _, ticket := range project.Tickets {
			if ticket.Broken {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s: %s does not parse; it is copied unchanged and still needs repair", project.Name, ticket.OldPath))
				continue
			}
			data, err := s.ReadFile(ticket.OldPath)
			if err != nil {
				return Plan{}, err
			}
			for _, dependency := range mdfile.Parse(data).List("depends-on") {
				if resolveSlug(dependency, project.Name, slugs) == dependency {
					plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s: %s depends on %s, which does not exist; it is kept as written", project.Name, ticket.ID, dependency))
				}
			}
		}
	}
	return plan, nil
}

func prepareProject(s *store.Store, name string, options Options) (Project, error) {
	project := Project{Name: name, ProjectFile: s.Exists(path.Join(name, "project.yaml"))}
	switch {
	case options.Keys[name] != "":
		project.Key, project.KeySource = options.Keys[name], "--key"
	default:
		if key := projectFileKey(s, name); key != "" {
			project.Key, project.KeySource = key, "project.yaml"
		} else {
			project.Key, project.KeySource = board.DeriveKey(name), "derived"
		}
	}

	type candidate struct {
		Ticket
		created string
	}
	var candidates []candidate
	collect := func(dir string, archived bool, status board.Column) error {
		entries, err := s.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			file := entry.Name()
			if entry.IsDir() || strings.HasPrefix(file, ".") || !strings.HasSuffix(file, ".md") {
				continue
			}
			oldPath := path.Join(dir, file)
			data, err := s.ReadFile(oldPath)
			if err != nil {
				return err
			}
			doc := mdfile.Parse(data)
			created, _ := doc.String("created")
			slug := strings.TrimSuffix(file, ".md")
			candidates = append(candidates, candidate{
				Ticket:  Ticket{OldPath: oldPath, OldSlug: slug, Status: status, Archived: archived, Broken: doc.FrontmatterError != nil},
				created: created,
			})
		}
		return nil
	}
	for _, column := range store.V1Columns {
		if err := collect(path.Join(name, column), false, v1Status[column]); err != nil {
			return Project{}, err
		}
		if err := collect(path.Join(name, ".archive", column), true, v1Status[column]); err != nil {
			return Project{}, err
		}
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		switch {
		case a.created == "" && b.created != "":
			return 1
		case b.created == "" && a.created != "":
			return -1
		}
		if c := strings.Compare(a.created, b.created); c != 0 {
			return c
		}
		return strings.Compare(a.OldSlug, b.OldSlug)
	})
	for index, candidate := range candidates {
		ticket := candidate.Ticket
		ticket.ID = fmt.Sprintf("%s-%d", project.Key, index+1)
		readable := typePrefix.ReplaceAllString(ticket.OldSlug, "")
		if readable == "" || !store.ValidName(readable) {
			readable = "ticket"
		}
		ticket.Folder = ticket.ID + "-" + readable
		if review := path.Join(name, "reviews", ticket.OldSlug+".md"); s.Exists(review) {
			ticket.Review = review
		}
		if files := path.Join(name, "attachments", ticket.OldSlug); s.IsDir(files) {
			ticket.Files = files
		}
		project.Tickets = append(project.Tickets, ticket)
	}
	project.NextID = len(project.Tickets) + 1

	if entries, err := s.ReadDir(path.Join(name, "workstreams")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") && !strings.HasPrefix(entry.Name(), ".") {
				project.Workstreams = append(project.Workstreams, path.Join(name, "workstreams", entry.Name()))
			}
		}
	}
	for _, dir := range append(slices.Clone(store.V1Columns), "reviews", "attachments") {
		if s.IsDir(path.Join(name, dir)) {
			project.Moves = append(project.Moves, path.Join(name, dir))
		}
	}
	for _, column := range store.V1Columns {
		if s.IsDir(path.Join(name, ".archive", column)) {
			project.Moves = append(project.Moves, path.Join(name, ".archive", column))
		}
	}
	return project, nil
}

func projectFileKey(s *store.Store, name string) string {
	data, err := s.ReadFile(path.Join(name, "project.yaml"))
	if err != nil {
		return ""
	}
	var parsed struct {
		Key string `yaml:"key"`
	}
	if yaml.Unmarshal(data, &parsed) != nil || !board.ValidKey(parsed.Key) {
		return ""
	}
	return parsed.Key
}

// resolveSlug maps a v1 reference ("slug" or "project/slug") to an id, or
// returns it unchanged when it names no migrated ticket.
func resolveSlug(reference, project string, slugs map[string]map[string]string) string {
	other, slug, found := strings.Cut(reference, "/")
	if !found {
		other, slug = project, reference
	}
	if id, ok := slugs[other][slug]; ok {
		return id
	}
	return reference
}

// Apply performs a prepared plan: it writes the v2 tree, then moves every
// v1 file and folder it replaced into the backup. Nothing is deleted.
func Apply(s *store.Store, plan Plan) error {
	if len(plan.Projects) == 0 {
		return nil
	}
	slugs := map[string]map[string]string{}
	folderOf := map[string]map[string]string{}
	for _, project := range plan.Projects {
		slugs[project.Name], folderOf[project.Name] = map[string]string{}, map[string]string{}
		for _, ticket := range project.Tickets {
			slugs[project.Name][ticket.OldSlug] = ticket.ID
			folderOf[project.Name][ticket.OldSlug] = ticket.Folder
		}
	}
	backup := func(name string) error {
		data, err := s.ReadFile(name)
		if err != nil {
			return err
		}
		return s.WriteFileAtomic(path.Join(plan.Backup, name), data)
	}

	for _, project := range plan.Projects {
		ids := func(value string) string { return resolveSlug(value, project.Name, slugs) }
		for _, ticket := range project.Tickets {
			data, err := s.ReadFile(ticket.OldPath)
			if err != nil {
				return err
			}
			target := ticket.Target(project.Name)
			if !ticket.Broken {
				data = insertFields(data, ticket.ID, ticket.Status)
				data = rewriteList(data, "depends-on", ids)
				data = []byte(rewriteLinks(string(data), ticket.Folder, folderOf[project.Name], slugs[project.Name]))
			}
			if err := s.WriteFileAtomic(path.Join(target, "ticket.md"), data); err != nil {
				return err
			}
			if ticket.Review != "" {
				review, err := s.ReadFile(ticket.Review)
				if err != nil {
					return err
				}
				rewritten := rewriteLinks(string(review), ticket.Folder, folderOf[project.Name], slugs[project.Name])
				if err := s.WriteFileAtomic(path.Join(target, "review.md"), []byte(rewritten)); err != nil {
					return err
				}
			}
			if ticket.Files != "" {
				if err := s.Move(ticket.Files, path.Join(target, "files")); err != nil {
					return err
				}
			}
		}
		for _, workstream := range project.Workstreams {
			data, err := s.ReadFile(workstream)
			if err != nil {
				return err
			}
			if err := backup(workstream); err != nil {
				return err
			}
			if err := s.WriteFileAtomic(workstream, rewriteList(data, "tickets", ids)); err != nil {
				return err
			}
		}
		projectFile := path.Join(project.Name, "project.yaml")
		var content []byte
		if project.ProjectFile {
			data, err := s.ReadFile(projectFile)
			if err != nil {
				return err
			}
			if err := backup(projectFile); err != nil {
				return err
			}
			content = setTopLevel(setTopLevel(data, "next_id", fmt.Sprint(project.NextID)), "key", project.Key)
		} else {
			content = []byte(fmt.Sprintf("key: %s\nnext_id: %d\n", project.Key, project.NextID))
		}
		if err := s.WriteFileAtomic(projectFile, content); err != nil {
			return err
		}
		for _, move := range project.Moves {
			if err := s.Move(move, path.Join(plan.Backup, move)); err != nil {
				return err
			}
		}
	}

	config := ".flashheart/config.yaml"
	if s.Exists(config) {
		data, err := s.ReadFile(config)
		if err != nil {
			return err
		}
		if err := backup(config); err != nil {
			return err
		}
		return s.WriteFileAtomic(config, setTopLevel(data, "version", "2"))
	}
	return s.WriteFileAtomic(config, []byte("version: 2\n"))
}

// insertFields puts id and status at the top of the frontmatter, adding a
// frontmatter block when there is none.
func insertFields(data []byte, id string, status board.Column) []byte {
	fields := fmt.Sprintf("id: %s\nstatus: %s\n", id, status)
	if bytes.HasPrefix(data, []byte("---\n")) {
		return append([]byte("---\n"+fields), data[4:]...)
	}
	return append([]byte("---\n"+fields+"---\n"), data...)
}

var flowList = regexp.MustCompile(`^\[(.*)\](\s*(#.*)?)$`)

// rewriteList maps the values of a top-level frontmatter list field (flow,
// block or scalar form), leaving everything else byte for byte.
func rewriteList(data []byte, field string, mapValue func(string) string) []byte {
	lines := strings.SplitAfter(string(data), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\n") != "---" {
		return data
	}
	mapOne := func(value string) string {
		trimmed := strings.TrimSpace(value)
		if mapped := mapValue(trimmed); mapped != "" && trimmed != "" {
			return strings.Replace(value, trimmed, mapped, 1)
		}
		return value
	}
	prefix := field + ":"
	for index := 1; index < len(lines); index++ {
		line := lines[index]
		if strings.TrimRight(line, "\n") == "---" {
			break
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimRight(strings.TrimPrefix(line, prefix), "\n")
		value := strings.TrimSpace(rest)
		switch {
		case value == "":
			for next := index + 1; next < len(lines); next++ {
				item := lines[next]
				trimmed := strings.TrimLeft(item, " ")
				if !strings.HasPrefix(trimmed, "- ") {
					break
				}
				indent := item[:len(item)-len(trimmed)]
				body := strings.TrimRight(trimmed[2:], "\n")
				comment := ""
				if at := strings.Index(body, " #"); at >= 0 {
					body, comment = body[:at], body[at:]
				}
				lines[next] = indent + "- " + mapOne(body) + comment + "\n"
			}
		case flowList.MatchString(value):
			match := flowList.FindStringSubmatch(value)
			var items []string
			for _, item := range strings.Split(match[1], ",") {
				if strings.TrimSpace(item) != "" {
					items = append(items, strings.TrimSpace(mapOne(item)))
				}
			}
			lines[index] = prefix + " [" + strings.Join(items, ", ") + "]" + match[2] + "\n"
		default:
			lines[index] = prefix + " " + mapOne(value) + "\n"
		}
		break
	}
	return []byte(strings.Join(lines, ""))
}

// setTopLevel replaces a top-level "key: value" line in a YAML document, or
// adds one at the top.
func setTopLevel(data []byte, key, value string) []byte {
	lines := strings.SplitAfter(string(data), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, key+":") {
			lines[index] = key + ": " + value + "\n"
			return []byte(strings.Join(lines, ""))
		}
	}
	return append([]byte(key+": "+value+"\n"), data...)
}

var markdownLink = regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^)\s]+)\)`)

// rewriteLinks points v1 links between tickets and to attachments at the v2
// folders, relative to a file inside tickets/<current>/.
func rewriteLinks(text, current string, folders, ids map[string]string) string {
	return markdownLink.ReplaceAllStringFunc(text, func(link string) string {
		match := markdownLink.FindStringSubmatch(link)
		bang, label, target := match[1], match[2], match[3]
		pathPart, fragment, _ := strings.Cut(target, "#")
		if fragment != "" {
			fragment = "#" + fragment
		}
		parts := strings.Split(pathPart, "/")
		if index := slices.Index(parts, "attachments"); index >= 0 && index+2 < len(parts) {
			folder, ok := folders[parts[index+1]]
			if !ok {
				return link
			}
			file := strings.Join(parts[index+2:], "/")
			newTarget := "files/" + file
			if folder != current {
				newTarget = "../" + folder + "/files/" + file
			}
			return bang + "[" + label + "](" + newTarget + fragment + ")"
		}
		if strings.Contains(pathPart, "://") || !strings.HasSuffix(pathPart, ".md") {
			return link
		}
		slug := strings.TrimSuffix(parts[len(parts)-1], ".md")
		folder, ok := folders[slug]
		if !ok {
			return link
		}
		newTarget := "../" + folder + "/ticket.md"
		if folder == current {
			newTarget = "ticket.md"
		}
		if label == slug {
			label = ids[slug]
		}
		return bang + "[" + label + "](" + newTarget + fragment + ")"
	})
}

// Write renders the plan for a dry run.
func (p Plan) Write(w io.Writer) {
	if len(p.Projects) == 0 {
		fmt.Fprintf(w, "Nothing to migrate: %s is already board format v2.\n", p.Root)
		return
	}
	fmt.Fprintf(w, "Migrating %s to board format v2.\n", p.Root)
	for _, project := range p.Projects {
		noun := "tickets"
		if len(project.Tickets) == 1 {
			noun = "ticket"
		}
		fmt.Fprintf(w, "\n%s → key %s (%s), %d %s, next id %s-%d\n", project.Name, project.Key, project.KeySource, len(project.Tickets), noun, project.Key, project.NextID)
		for _, ticket := range project.Tickets {
			old := strings.TrimPrefix(ticket.OldPath, project.Name+"/")
			fmt.Fprintf(w, "  %-5s %s → %s (%s)\n", ticket.ID, old, strings.TrimPrefix(ticket.Target(project.Name), project.Name+"/"), ticket.Status)
		}
		for _, workstream := range project.Workstreams {
			fmt.Fprintf(w, "  workstream ticket lists rewritten: %s\n", strings.TrimPrefix(workstream, project.Name+"/"))
		}
	}
	if len(p.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, warning := range p.Warnings {
			fmt.Fprintf(w, "  %s\n", warning)
		}
	}
	fmt.Fprintf(w, "\nThe v1 files move to %s. Nothing is deleted.\n", p.Backup)
	fmt.Fprintln(w, "Run again with --write to apply.")
}
