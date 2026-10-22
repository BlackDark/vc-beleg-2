import { useQuery } from "@tanstack/react-query";
import { getSessionOptions } from "@/api/gen/@tanstack/react-query.gen";

export const sessionQuery = getSessionOptions();

export function useSession() {
  return useQuery(sessionQuery);
}

/** The logged-in user; routes below the authenticated layout guarantee it. */
export function useUser() {
  const { data } = useSession();
  if (!data?.user) throw new Error("not logged in");
  return data.user;
}
