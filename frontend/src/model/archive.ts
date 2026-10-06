// Archive view helpers (EDIT-8): search across archived tickets and the
// typed-id gate of a permanent delete.
import type { Archived } from "../api/archive";

export function filterArchived(items: Archived[], query: string): Archived[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return items;
  return items.filter((item) => {
    const text = [item.id, item.title, item.type, item.workstream]
      .join(" ")
      .toLowerCase();
    return words.every((word) => text.includes(word));
  });
}

// confirmsDelete is true only when the user typed the ticket's exact id.
export function confirmsDelete(typed: string, id: string): boolean {
  return typed.trim() === id;
}
