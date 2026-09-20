"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Plus, X } from "lucide-react";

import { usersApi, ALL_ROLES } from "@/lib/api/users";
import type { Role } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

const createUserSchema = z.object({
  username: z.string().min(3, "At least 3 characters"),
  fullName: z.string().min(1, "Required"),
  email: z.string().email("Enter a valid email"),
  password: z.string().min(8, "At least 8 characters"),
  roles: z.array(z.string()).min(1, "Select at least one role"),
});

type CreateUserForm = z.infer<typeof createUserSchema>;

export default function UsersAdminPage() {
  const [showForm, setShowForm] = useState(false);
  const queryClient = useQueryClient();

  const { data: userList, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: usersApi.list,
  });

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    setError,
    formState: { errors },
  } = useForm<CreateUserForm>({
    resolver: zodResolver(createUserSchema),
    defaultValues: { roles: [] },
  });

  const selectedRoles = watch("roles") as Role[];

  const toggleRole = (role: Role) => {
    const current = selectedRoles || [];
    if (current.includes(role)) {
      setValue(
        "roles",
        current.filter((r) => r !== role)
      );
    } else {
      setValue("roles", [...current, role]);
    }
  };

  const createMutation = useMutation({
    mutationFn: usersApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      reset({ username: "", fullName: "", email: "", password: "", roles: [] });
      setShowForm(false);
    },
    onError: (err) => {
      if (err instanceof ApiError && err.details) {
        for (const [field, message] of Object.entries(err.details)) {
          setError(field as keyof CreateUserForm, { message });
        }
      } else if (err instanceof ApiError) {
        setError("root", { message: err.message });
      }
    },
  });

  const toggleActiveMutation = useMutation({
    mutationFn: ({ id, isActive }: { id: string; isActive: boolean }) =>
      usersApi.update(id, { isActive }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">Users</h1>
          <p className="mt-1 text-sm text-slate-500">Manage who can access IT Operations Hub.</p>
        </div>
        <Button onClick={() => setShowForm((s) => !s)}>
          {showForm ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
          {showForm ? "Cancel" : "New User"}
        </Button>
      </div>

      {showForm && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create User</CardTitle>
            <CardDescription>New users are created active with the roles you assign.</CardDescription>
          </CardHeader>
          <CardContent>
            <form
              className="space-y-4"
              onSubmit={handleSubmit((data) => createMutation.mutate(data as never))}
            >
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="username">Username</Label>
                  <Input id="username" {...register("username")} />
                  {errors.username && <p className="text-xs text-red-600">{errors.username.message}</p>}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="fullName">Full Name</Label>
                  <Input id="fullName" {...register("fullName")} />
                  {errors.fullName && <p className="text-xs text-red-600">{errors.fullName.message}</p>}
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="email">Email</Label>
                  <Input id="email" type="email" {...register("email")} />
                  {errors.email && <p className="text-xs text-red-600">{errors.email.message}</p>}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="password">Temporary Password</Label>
                  <Input id="password" type="password" {...register("password")} />
                  {errors.password && <p className="text-xs text-red-600">{errors.password.message}</p>}
                </div>
              </div>

              <div className="space-y-1.5">
                <Label>Roles</Label>
                <div className="flex flex-wrap gap-2">
                  {ALL_ROLES.map((role) => (
                    <button
                      type="button"
                      key={role}
                      onClick={() => toggleRole(role)}
                      className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                        selectedRoles?.includes(role)
                          ? "border-indigo-600 bg-indigo-600 text-white"
                          : "border-slate-300 bg-white text-slate-600 hover:bg-slate-50"
                      }`}
                    >
                      {role}
                    </button>
                  ))}
                </div>
                {errors.roles && <p className="text-xs text-red-600">{errors.roles.message as string}</p>}
              </div>

              {errors.root && <p className="text-xs text-red-600">{errors.root.message}</p>}

              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Create User"}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="flex justify-center py-10">
              <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left text-xs font-semibold uppercase tracking-wider text-slate-400">
                  <th className="px-6 py-3">Name</th>
                  <th className="px-6 py-3">Username</th>
                  <th className="px-6 py-3">Email</th>
                  <th className="px-6 py-3">Roles</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3"></th>
                </tr>
              </thead>
              <tbody>
                {userList?.map((u) => (
                  <tr key={u.id} className="border-b border-slate-100 last:border-0">
                    <td className="px-6 py-3 font-medium text-slate-900">{u.fullName}</td>
                    <td className="px-6 py-3 text-slate-600">{u.username}</td>
                    <td className="px-6 py-3 text-slate-600">{u.email}</td>
                    <td className="px-6 py-3">
                      <div className="flex flex-wrap gap-1">
                        {u.roles.map((r) => (
                          <span
                            key={r}
                            className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600"
                          >
                            {r}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className="px-6 py-3">
                      <span
                        className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                          u.isActive ? "bg-emerald-100 text-emerald-700" : "bg-slate-200 text-slate-500"
                        }`}
                      >
                        {u.isActive ? "Active" : "Inactive"}
                      </span>
                    </td>
                    <td className="px-6 py-3 text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() =>
                          toggleActiveMutation.mutate({ id: u.id, isActive: !u.isActive })
                        }
                        disabled={toggleActiveMutation.isPending}
                      >
                        {u.isActive ? "Deactivate" : "Activate"}
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
