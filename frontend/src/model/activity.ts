// A ticket's activity for its full page (CARD-7, FH-65): its notes, the
// agents' questions about it with their answers, and its handoff updates,
// as one timeline, oldest first. Everything comes from what the ticket
// API already returns: the notes are the ticket's ## Notes bullets
// (board.NoteLine), the questions are its open questions and those its
// linked runs asked about it, and the handoff updates are those runs'
// checkpoints on it plus the current handoff's _Updated_ line. Runs cover
// the server's run window (48 hours), so an older question or checkpoint
// survives only as its note or as the current handoff (D46).
import type { TicketDetail } from "../api/board";
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
  // state: open (waiting, shown above as well), answered on the board, or
  // answered in the session's own chat.
  state: "open" | "answered" | "in-session";
}

export interface HandoffEntry extends Dated {
  kind: "handoff";
  time: string;
  by: string;
  // next are the next steps of the handoff it wrote, for the current one.
  next: string[];
}

export type ActivityEntry = NoteEntry | QuestionEntry | HandoffEntry;

export type ActivityInput = Pick<
  TicketDetail,
  "id" | "body" | "handoff" | "runs" | "questions"
>;

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
  const asked = [...questions.values()].map(questionEntry);
  // A question answered on the board is also in the notes; the run's copy
  // has the times, so its note is left out.
  const notes = notesOf(detail.body).filter(
    (note) =>
      note.kind !== "question" ||
      !asked.some(
        (entry) => entry.by === note.by && entry.question === note.question,
      ),
  );
  const timed = [...asked, ...handoffsOf(detail)].sort((a, b) =>
    compareTimes(a.time, b.time),
  );
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
