// A ticket's activity for its full page (CARD-7, FH-65, FH-125): people's
// comments, its notes, its history, the agents' questions about it with
// their answers, and its handoff updates, as one timeline, oldest first.
// Everything comes from what the ticket API returns: comments and history
// are its activity file (STO-8, D47), kept for the ticket's life; the notes
// are the ticket's ## Notes bullets (board.NoteLine); the questions are
// those its history keeps (FH-190) with its open questions and those its
// linked runs asked about it, once each; and the handoff updates are those
// runs' checkpoints on it plus the current handoff's _Updated_ line. Runs
// cover the server's run window (48 hours), so an older checkpoint
// survives only as the current handoff (D46), and an older question as its
// history or its note; agents' moves from the runs fill in only before the
// activity file's history begins, so no move shows twice.
import {
  type Comment,
  type CommentDelivery,
  type History,
  type HistoryChange,
  STATUSES,
  type TicketDetail,
} from "../api/board";
import type { Question, QuestionKind } from "../api/runs";
import { markdownSections } from "./markdown";
import { compareTimes } from "./time";

interface Dated {
  // key is stable for one entry across reloads.
  key: string;
  // day is the local date (YYYY-MM-DD), the only time a note has; "" for
  // undated text.
  day: string;
}

export interface NoteEntry extends Dated {
  kind: "note";
  // by is the author's short run id, "" for a note the board wrote.
  by: string;
  markdown: string;
}

export interface QuestionEntry extends Dated {
  kind: "question";
  // time is when it was asked, "" when only its note's day is known.
  time: string;
  by: string;
  questionKind?: QuestionKind;
  question: string;
  answer: string;
  answeredBy: string;
  answeredAt: string;
  // state: open (waiting, shown above as well), answered on the board,
  // answered in the session's own chat, or, from the ticket's history
  // alone (FH-190), left open when its session ended, or with no outcome
  // recorded.
  state: QuestionState;
}

export type QuestionState =
  | "open"
  | "answered"
  | "in-session"
  | "ended"
  | "unanswered";

export interface HandoffEntry extends Dated {
  kind: "handoff";
  time: string;
  by: string;
  // next are the next steps of the handoff it wrote, for the current one.
  next: string[];
}

// CommentEntry is a person's comment (STO-8): edited is when it was last
// edited ("" if never), quote the part of the ticket it quotes, from
// quoteFrom.
export interface CommentEntry extends Dated {
  kind: "comment";
  time: string;
  id: string;
  by: string;
  byPerson: boolean;
  edited: string;
  // state is the comment's state as a change request (REV-6): open for
  // a plain comment, change-requested, addressed or resolved.
  state: string;
  // delivery is where a comment sent to a session stands (FH-127), ""
  // for one not sent; sentTo names the run, short, and delivered is when
  // it was handed over.
  delivery: CommentDelivery | "";
  sentTo: string;
  delivered: string;
  quoteFrom: string;
  quote: string;
  markdown: string;
}

// HistoryEntry is one write that changed the ticket, its changes in words
// (historyWords).
export interface HistoryEntry extends Dated {
  kind: "history";
  time: string;
  by: string;
  byPerson: boolean;
  words: string[];
}

export type ActivityEntry =
  | NoteEntry
  | QuestionEntry
  | HandoffEntry
  | CommentEntry
  | HistoryEntry;

export type ActivityInput = Pick<
  TicketDetail,
  "id" | "body" | "handoff" | "runs" | "questions" | "comments" | "history"
>;

// ActivityFilter is what Activity shows: everything, people's comments, or
// the ticket's history of changes (D47).
export type ActivityFilter = "all" | "comments" | "history";

export const ACTIVITY_FILTERS: { value: ActivityFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "comments", label: "Comments" },
  { value: "history", label: "History" },
];

// filterActivity keeps the entries a filter shows.
export function filterActivity(
  entries: ActivityEntry[],
  filter: ActivityFilter,
): ActivityEntry[] {
  if (filter === "comments")
    return entries.filter((entry) => entry.kind === "comment");
  // A question is part of the ticket's history (FH-190).
  if (filter === "history")
    return entries.filter(
      (entry) => entry.kind === "history" || entry.kind === "question",
    );
  return entries;
}

const statusTitles = new Map<string, string>(
  STATUSES.map((status) => [status.id, status.title]),
);
const statusTitle = (status: string) => statusTitles.get(status) ?? status;

// The names a history entry gives what was edited (board-format
// §Activity), in words.
const fieldWords: Record<string, string> = {
  title: "the title",
  description: "the description",
  "acceptance-criteria": "the acceptance criteria",
  "test-plan": "the test plan",
  reproduction: "the reproduction",
  context: "the context",
  review: "the review",
  body: "the ticket's text",
  frontmatter: "the frontmatter",
  pr: "the pull request",
  "depends-on": "its dependencies",
  "depends-on-workstreams": "its workstream dependencies",
};
const fieldWord = (field: string) =>
  fieldWords[field] ?? `the ${field.replaceAll("-", " ")}`;

// andList joins words as a sentence does: "a, b and c".
export function andList(items: string[]): string {
  if (items.length <= 1) return items[0] ?? "";
  return `${items.slice(0, -1).join(", ")} and ${items.at(-1)}`;
}

// numbered names ticked items: "criterion 2", "review steps 1 and 3".
function numbered(one: string, many: string, numbers: number[]): string[] {
  if (numbers.length === 0) return [];
  return [
    `${numbers.length === 1 ? one : many} ${andList(numbers.map(String))}`,
  ];
}

// counted names a comment state change (FH-126) by how many comments it
// covered: "asked for a change", "addressed 2 change requests".
function counted(
  change: HistoryChange,
  one: string,
  verb: string,
  many: string,
): string {
  const n = change.comments?.length ?? 0;
  return n > 1 ? `${verb} ${n} ${many}` : one;
}

// historyWords says what each change of a write did, in plain words and
// never with the ticket's text.
export function historyWords(changes: HistoryChange[]): string[] {
  return changes.map((change) => {
    switch (change.what) {
      case "created":
        return change.to
          ? `created it in ${statusTitle(change.to)}`
          : "created it";
      case "moved":
        return change.from
          ? `moved it from ${statusTitle(change.from)} to ${statusTitle(change.to)}`
          : `moved it to ${statusTitle(change.to)}`;
      case "edited":
        return `edited ${andList(change.fields.map(fieldWord))}`;
      case "ticked":
      case "unticked":
        return `${change.what} ${andList([
          ...numbered("criterion", "criteria", change.criteria),
          ...numbered("review step", "review steps", change.steps),
        ])}`;
      case "archived":
        return "archived it";
      case "restored":
        return "restored it";
      case "change-requested":
        return counted(change, "asked for a change", "asked for", "changes");
      case "addressed":
        return counted(
          change,
          "addressed a change request",
          "addressed",
          "change requests",
        );
      case "resolved":
        return counted(
          change,
          "resolved a change request",
          "resolved",
          "change requests",
        );
      case "reopened":
        return counted(
          change,
          "reopened a change request",
          "reopened",
          "change requests",
        );
      case "deleted":
        return counted(change, "deleted a comment", "deleted", "comments");
      default:
        return change.what;
    }
  });
}

const noteBullet = /^- (\d{4}-\d{2}-\d{2}) · ([\s\S]*)$/;
const authored = /^([a-z][a-z0-9-]*:[A-Za-z0-9…-]+): ([\s\S]*)$/;
const quoted = String.raw`"(?:[^"\\]|\\.)*"`;
const answeredNote = new RegExp(
  String.raw`^Question from (\S+) — (${quoted}) Answer \((.*?)\): (${quoted})\.?$`,
);
const handoffLine =
  /_Updated (\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}) UTC by (.+?) \(run\)(?: on (.+?))?\._/;

// unquote reads a Go %q string, as the board writes a question and its
// answer into the note.
function unquote(text: string): string {
  try {
    return JSON.parse(text) as string;
  } catch {
    return text.slice(1, -1);
  }
}

// localDay is an instant's date where the browser is, as notes are dated
// where the board runs.
export function localDay(time: string): string {
  const date = new Date(time);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

// notesOf reads the ## Notes section: each dated bullet is one entry, its
// continuation lines with it; text before the first bullet is one undated
// note unless it only says None.
function notesOf(body: string): (NoteEntry | QuestionEntry)[] {
  const notes = markdownSections(body).sections.find(
    (section) => section.title.trim().toLowerCase() === "notes",
  );
  if (!notes) return [];
  const chunks: string[][] = [[]];
  for (const line of notes.markdown.split("\n")) {
    if (line.startsWith("- ")) chunks.push([line]);
    else chunks[chunks.length - 1]?.push(line);
  }
  const entries: (NoteEntry | QuestionEntry)[] = [];
  chunks.forEach((lines, index) => {
    const text = lines.join("\n").trim();
    if (!text) return;
    const key = `note:${entries.length}`;
    const bullet = index > 0 ? noteBullet.exec(text) : null;
    if (!bullet) {
      if (index === 0 && /^none\.?$/i.test(text)) return;
      entries.push({ kind: "note", key, day: "", by: "", markdown: text });
      return;
    }
    const day = bullet[1] ?? "";
    const rest = bullet[2] ?? "";
    const answer = answeredNote.exec(rest);
    if (answer) {
      entries.push({
        kind: "question",
        key,
        day,
        time: "",
        by: answer[1] ?? "",
        question: unquote(answer[2] ?? '""'),
        answer: unquote(answer[4] ?? '""'),
        answeredBy: answer[3] ?? "",
        answeredAt: "",
        state: "answered",
      });
      return;
    }
    const author = authored.exec(rest);
    entries.push({
      kind: "note",
      key,
      day,
      by: author?.[1] ?? "",
      markdown: (author ? author[2] : rest)?.trim() ?? "",
    });
  });
  return entries;
}

function questionEntry(question: Question): QuestionEntry {
  return {
    kind: "question",
    key: `question:${question.id}`,
    day: localDay(question.asked),
    time: question.asked,
    by: question.runShort,
    questionKind: question.kind,
    question: question.text,
    answer: question.answer ?? "",
    answeredBy: question.answeredBy ?? "",
    answeredAt: question.answeredAt ?? "",
    state: question.answer
      ? "answered"
      : question.answeredInSession
        ? "in-session"
        : "open",
  };
}

// The history's question changes (FH-190, board-format §Activity).
const questionChanges = new Set([
  "asked",
  "answered",
  "answered-in-session",
  "session-ended",
]);
const isQuestionChange = (change: HistoryChange) =>
  questionChanges.has(change.what) && change.question !== undefined;

// questionsKept are the questions the ticket's history keeps, by id, each
// with what became of it: asked (by the asking run, when), then answered
// on the board (by the person, when), in the session's chat, or left open
// when its session ended.
function questionsKept(history: History[]): Map<string, QuestionEntry> {
  const kept = new Map<string, QuestionEntry>();
  for (const entry of history) {
    for (const change of entry.changes) {
      const q = change.question;
      if (!q || !isQuestionChange(change)) continue;
      let question = kept.get(q.id);
      if (!question) {
        question = {
          kind: "question",
          key: `question:${q.id}`,
          day: localDay(entry.time),
          time: entry.time,
          by: entry.byPerson ? "" : entry.byShort,
          question: q.text,
          answer: "",
          answeredBy: "",
          answeredAt: "",
          state: "unanswered",
        };
        kept.set(q.id, question);
      }
      if (change.what === "asked") {
        question.day = localDay(entry.time);
        question.time = entry.time;
        question.by = entry.byShort;
        question.question = q.text || question.question;
        if (q.kind) question.questionKind = q.kind as QuestionKind;
      } else if (!question.question) question.question = q.text;
      if (change.what === "answered") {
        question.state = "answered";
        question.answer = q.answer;
        question.answeredBy = entry.byShort;
        question.answeredAt = entry.time;
      } else if (
        change.what === "answered-in-session" &&
        question.state !== "answered"
      )
        question.state = "in-session";
      else if (
        change.what === "session-ended" &&
        question.state === "unanswered"
      )
        question.state = "ended";
    }
  }
  return kept;
}

// How much a question's state says, so of a question both the history and
// the run window have, the one that knows more is shown.
const stateRank: Record<QuestionState, number> = {
  unanswered: 0,
  ended: 1,
  open: 2,
  "in-session": 3,
  answered: 4,
};

// handoffsOf lists the checkpoints the ticket's runs made on it, and the
// current handoff, whose next steps go with the checkpoint that wrote it
// (the same run, the same minute).
function handoffsOf(detail: ActivityInput): HandoffEntry[] {
  const seen = new Set<string>();
  const entries: HandoffEntry[] = [];
  for (const run of detail.runs) {
    for (const entry of run.timeline ?? []) {
      if (entry.kind !== "checkpoint" || entry.ticket !== detail.id) continue;
      const key = `handoff:${run.short}:${entry.time}`;
      if (seen.has(key)) continue;
      seen.add(key);
      entries.push({
        kind: "handoff",
        key,
        day: localDay(entry.time),
        time: entry.time,
        by: run.short,
        next: [],
      });
    }
  }
  const line = detail.handoff
    ? handoffLine.exec(detail.handoff.markdown)
    : null;
  if (!detail.handoff || !line) return entries;
  const time = `${line[1]}T${line[2]}:00Z`;
  const by = line[3] ?? "";
  const minute = time.slice(0, 16);
  const same = (entry: HandoffEntry) =>
    new Date(entry.time).toISOString().slice(0, 16) === minute &&
    (entry.by === by || by.startsWith(entry.by) || entry.by.startsWith(by));
  const current = entries.find(same);
  if (current) current.next = detail.handoff.next;
  else
    entries.push({
      kind: "handoff",
      key: "handoff:current",
      day: localDay(time),
      time,
      by,
      next: detail.handoff.next,
    });
  return entries;
}

function commentEntry(comment: Comment): CommentEntry {
  return {
    kind: "comment",
    key: `comment:${comment.id}`,
    day: localDay(comment.time),
    time: comment.time,
    id: comment.id,
    by: comment.byShort,
    byPerson: comment.byPerson,
    edited: comment.edited,
    state: comment.state,
    delivery: comment.delivery ?? "",
    sentTo: comment.sentToShort ?? "",
    delivered: comment.delivered ?? "",
    quoteFrom: comment.quoteFrom,
    quote: comment.quote,
    markdown: comment.body,
  };
}

// historyOf is the activity file's history, and before it begins the
// agents' moves and new tickets the runs recorded: a ticket without the
// file, or older than it, still shows what the run window holds, and a
// write the file records shows once.
function historyOf(detail: ActivityInput): HistoryEntry[] {
  // A question's changes show as the question (questionsKept).
  const durable = detail.history.flatMap((entry): HistoryEntry[] => {
    const changes = entry.changes.filter((change) => !isQuestionChange(change));
    if (changes.length === 0) return [];
    return [
      {
        kind: "history",
        key: `history:${entry.id}`,
        day: localDay(entry.time),
        time: entry.time,
        by: entry.byShort,
        byPerson: entry.byPerson,
        words: historyWords(changes),
      },
    ];
  });
  const begins = Math.min(
    ...detail.history.map((entry) => Date.parse(entry.time) || Infinity),
  );
  const seen = new Set<string>();
  const fromRuns: HistoryEntry[] = [];
  for (const run of detail.runs) {
    for (const entry of run.timeline ?? []) {
      if (entry.ticket !== detail.id || !(Date.parse(entry.time) < begins))
        continue;
      const what =
        entry.kind === "ticket.moved"
          ? "moved"
          : entry.kind === "ticket.created"
            ? "created"
            : "";
      const key = `history:${run.short}:${entry.kind}:${entry.time}`;
      if (!what || seen.has(key)) continue;
      seen.add(key);
      fromRuns.push({
        kind: "history",
        key,
        day: localDay(entry.time),
        time: entry.time,
        by: run.short,
        byPerson: false,
        words: historyWords([
          {
            what,
            from: "",
            to: what === "moved" ? (entry.detail ?? "") : "",
            fields: [],
            criteria: [],
            steps: [],
          },
        ]),
      });
    }
  }
  return [...fromRuns, ...durable];
}

// activityOf is the ticket's activity, oldest first: by day, and within a
// day the notes (which carry only a day) in their order, then the timed
// entries by time.
export function activityOf(detail: ActivityInput): ActivityEntry[] {
  const questions = new Map<string, Question>();
  for (const question of [
    ...detail.questions,
    ...detail.runs.flatMap((run) => run.questions ?? []),
  ]) {
    if (question.ticket === detail.id && !questions.has(question.id))
      questions.set(question.id, question);
  }
  // A question the history keeps and the run window shows is shown once,
  // as whichever knows more of what became of it.
  const kept = questionsKept(detail.history);
  const asked = [...questions.values()].map((question) => {
    const live = questionEntry(question);
    const durable = kept.get(question.id);
    kept.delete(question.id);
    if (!durable || stateRank[live.state] >= stateRank[durable.state])
      return {
        ...live,
        questionKind: live.questionKind ?? durable?.questionKind,
      };
    return durable;
  });
  asked.push(...kept.values());
  // A question answered on the board is also in the notes; the run's copy
  // has the times, so its note is left out.
  const notes = notesOf(detail.body).filter(
    (note) =>
      note.kind !== "question" ||
      !asked.some(
        (entry) => entry.by === note.by && entry.question === note.question,
      ),
  );
  const timed = [
    ...historyOf(detail),
    ...asked,
    ...detail.comments.map(commentEntry),
    ...handoffsOf(detail),
  ].sort((a, b) => compareTimes(a.time, b.time));
  const all: ActivityEntry[] = [...notes, ...timed];
  return all
    .map((entry, index) => ({ entry, index }))
    .sort(
      (a, b) =>
        a.entry.day.localeCompare(b.entry.day) ||
        Number(isTimed(a.entry)) - Number(isTimed(b.entry)) ||
        a.index - b.index,
    )
    .map(({ entry }) => entry);
}

const isTimed = (entry: ActivityEntry) =>
  entry.kind !== "note" && entry.time !== "";
