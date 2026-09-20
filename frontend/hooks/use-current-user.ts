"use client";

import { useQuery } from "@tanstack/react-query";
import { authApi } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";

export function useCurrentUser() {
  return useQuery({
    queryKey: ["auth", "me"],
    queryFn: authApi.me,
    retry: false,
    // A 401 is an expected "not logged in" state, not a transient failure —
    // don't spam the backend retrying it.
    throwOnError: (error) => !(error instanceof ApiError && error.status === 401),
  });
}
