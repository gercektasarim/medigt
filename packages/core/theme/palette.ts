// Available color palettes. Health-themed default is "teal" — calming and clinical.
// Switch palettes via <html data-palette="...">; see packages/ui/styles/palettes.css.

export const PALETTES = ["teal", "blue", "indigo", "violet", "rose", "amber", "green", "slate"] as const;
export type Palette = (typeof PALETTES)[number];

export const DEFAULT_PALETTE: Palette = "teal";

export function isPalette(value: string): value is Palette {
  return (PALETTES as readonly string[]).includes(value);
}
