export type ReportColumnType =
  | "text"
  | "number"
  | "currency"
  | "date"
  | "datetime"
  | "pct";

export type ReportColumn = {
  key: string;
  label: string;
  type: ReportColumnType;
  align?: "right" | "left" | "center";
};

export type ReportResult = {
  columns: ReportColumn[];
  rows: Record<string, unknown>[];
  summary?: Record<string, unknown>;
};

// Frontend descriptor — registry used by the rapor hub.
export type ReportParam = {
  key: string;
  label: string;
  type: "date" | "select" | "number" | "text";
  options?: { value: string; label: string }[];
  default?: string | number;
  required?: boolean;
};

export type ReportDescriptor = {
  id: string;     // matches backend registry key
  slug: string;   // URL slug
  group: string;  // grouping label in the hub
  title: string;
  description?: string;
  params: ReportParam[];
};
