import { ListEnvelope } from "@/types/api";

// Selectors need the complete visible set; every HTTP response remains bounded.
export async function fetchAllPages<T>(fetchPage: (offset: number) => Promise<ListEnvelope<T>>): Promise<ListEnvelope<T>> {
  const items: T[] = [];
  let offset = 0;
  for (;;) {
    const page = await fetchPage(offset);
    items.push(...page.items);
    if (page.next_offset == null) return { items };
    if (page.next_offset <= offset || page.next_offset > 1_000_000) {
      throw new Error("Invalid pagination response");
    }
    offset = page.next_offset;
  }
}
