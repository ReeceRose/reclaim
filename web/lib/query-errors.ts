import { ApiError } from '@/lib/api';

/** Human-readable message for query/mutation failures (never raw status codes). */
export function apiErrorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return 'Something went wrong. Try again.';
}
