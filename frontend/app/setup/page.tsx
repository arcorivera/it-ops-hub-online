"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ShieldCheck, Loader2 } from "lucide-react";

import { authApi } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

const setupSchema = z
  .object({
    username: z.string().min(3, "Username must be at least 3 characters"),
    fullName: z.string().min(1, "Full name is required"),
    email: z.string().email("Enter a valid email address"),
    password: z.string().min(8, "Password must be at least 8 characters"),
    confirmPassword: z.string(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Passwords do not match",
    path: ["confirmPassword"],
  });

type SetupForm = z.infer<typeof setupSchema>;

export default function SetupPage() {
  const router = useRouter();

  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ["auth", "status"],
    queryFn: authApi.status,
  });

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<SetupForm>({ resolver: zodResolver(setupSchema) });

  const mutation = useMutation({
    mutationFn: authApi.setup,
    onSuccess: () => {
      router.push("/dashboard");
    },
    onError: (err) => {
      if (err instanceof ApiError && err.details) {
        for (const [field, message] of Object.entries(err.details)) {
          setError(field as keyof SetupForm, { message });
        }
      } else if (err instanceof ApiError) {
        setError("root", { message: err.message });
      }
    },
  });

  useEffect(() => {
    // If setup has already been completed, don't let anyone re-run it —
    // send them to login instead. The backend also rejects this server-side.
    if (!statusLoading && status && !status.needsSetup) {
      router.replace("/login");
    }
  }, [status, statusLoading, router]);

  if (statusLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50">
        <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
      <div className="w-full max-w-md">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-indigo-600">
            <ShieldCheck className="h-6 w-6 text-white" />
          </div>
          <h1 className="text-2xl font-semibold text-slate-900">IT Operations Hub</h1>
          <p className="text-sm text-slate-500">Everything that needs your attention, in one place.</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Create Administrator</CardTitle>
            <CardDescription>
              No users exist yet. Set up the first administrator account to get started.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-4" onSubmit={handleSubmit((data) => mutation.mutate(data))}>
              <div className="space-y-1.5">
                <Label htmlFor="username">Username</Label>
                <Input id="username" autoComplete="username" {...register("username")} />
                {errors.username && <p className="text-xs text-red-600">{errors.username.message}</p>}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="fullName">Full Name</Label>
                <Input id="fullName" autoComplete="name" {...register("fullName")} />
                {errors.fullName && <p className="text-xs text-red-600">{errors.fullName.message}</p>}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="email">Email</Label>
                <Input id="email" type="email" autoComplete="email" {...register("email")} />
                {errors.email && <p className="text-xs text-red-600">{errors.email.message}</p>}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="password">Password</Label>
                <Input id="password" type="password" autoComplete="new-password" {...register("password")} />
                {errors.password && <p className="text-xs text-red-600">{errors.password.message}</p>}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="confirmPassword">Confirm Password</Label>
                <Input id="confirmPassword" type="password" autoComplete="new-password" {...register("confirmPassword")} />
                {errors.confirmPassword && (
                  <p className="text-xs text-red-600">{errors.confirmPassword.message}</p>
                )}
              </div>

              {errors.root && <p className="text-xs text-red-600">{errors.root.message}</p>}

              <Button type="submit" className="w-full" disabled={mutation.isPending}>
                {mutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Create Administrator"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
