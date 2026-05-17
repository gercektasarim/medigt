export type Uuid = string;

export type Timestamps = {
  created_at: string;
  updated_at: string;
};

export type PaginatedResult<T> = {
  items: T[];
  total: number;
  page: number;
  page_size: number;
};

export type ApiError = {
  code: string;
  message: string;
  details?: Record<string, unknown>;
};
