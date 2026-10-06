// Staff and roles as the People screen reads and writes them
// (backoffice-people.md). Every write reads the list again.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { PeopleResponse, PersonInput, PinRequest, RoleEditResponse, RoleInput, StaffAddedResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";

export const peopleKey = ["people"] as const;

export function usePeople() {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const list = useQuery({
    queryKey: peopleKey,
    queryFn: ({ signal }) => store.api.get<PeopleResponse>("/api/v1/people", { signal }),
  });
  const after = async <T>(p: Promise<T>): Promise<T> => {
    try {
      return await p;
    } finally {
      await queryClient.invalidateQueries({ queryKey: peopleKey });
    }
  };
  const staff = (id: string) => `/api/v1/people/staff/${encodeURIComponent(id)}`;

  return {
    list,
    addStaff: (input: PersonInput) => after(store.api.post<StaffAddedResponse>("/api/v1/people/staff", input)).then((r) => r.id),
    updateStaff: (id: string, input: PersonInput) => after(store.api.put<StaffAddedResponse>(staff(id), input)).then(() => undefined),
    setActive: (id: string, active: boolean) =>
      after(store.api.post<StaffAddedResponse>(`${staff(id)}/${active ? "reactivate" : "deactivate"}`)).then(() => undefined),
    resetPIN: (id: string, pin: string) => {
      const body: PinRequest = { pin };
      return after(store.api.put<StaffAddedResponse>(`${staff(id)}/pin`, body)).then(() => undefined);
    },
    unlock: (id: string) => after(store.api.post<StaffAddedResponse>(`${staff(id)}/unlock`)).then(() => undefined),
    addRole: (input: RoleInput) => after(store.api.post<RoleEditResponse>("/api/v1/people/roles", input)).then((r) => r.id),
    updateRole: (id: number, input: RoleInput) => after(store.api.put<RoleEditResponse>(`/api/v1/people/roles/${id}`, input)).then(() => undefined),
    deleteRole: (id: number) => after(store.api.del<RoleEditResponse>(`/api/v1/people/roles/${id}`)).then(() => undefined),
  };
}

export type PeopleApi = ReturnType<typeof usePeople>;
