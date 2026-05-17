import type { Uuid } from "./common";

export type Icd10Code = {
  id: Uuid;
  code: string;
  title_tr: string;
  title_en?: string;
  parent_code?: string;
  chapter?: string;
  is_active: boolean;
  is_system: boolean;
  created_at: string;
};
