import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useUser } from '@/contexts/UserContext';
import { useI18n } from '@/contexts/I18nContext';
import { useLoginMutation } from '@/hooks/useAuth';
import api from '@/lib/api';
import type { Envelope, UserProfileResponse } from '@/types/api';
import { zodResolver } from '@hookform/resolvers/zod';
import { Eye, EyeOff, Lock, Mail, User } from "lucide-react";
import { useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { Link, useNavigate } from 'react-router-dom';
import { toast } from "sonner";
import * as z from 'zod';

// Main Login Component
const LoginPage: React.FC = () => {
  const { t } = useI18n();
  const [showPassword, setShowPassword] = useState(false);
  const [mode, setMode] = useState("login"); // login or forgotPassword
  const navigate = useNavigate();
  const { setUser } = useUser();
  // Login mutation
  const loginMutation = useLoginMutation();

  const loginSchema = useMemo(() => z.object({
    identifier: z.string()
      .min(1, t("auth.usernameOrEmail"))
      .min(3, t("auth.usernameOrEmail"))
      .max(96, t("auth.usernameOrEmail")),
    password: z.string()
      .min(1, t("auth.password"))
      .min(8, t("auth.password"))
      .max(72, t("auth.password")),
  }), [t]);

  // Form schema for password reset
  const resetSchema = useMemo(() => z.object({
    identifier: z.string().min(1, t("auth.usernameOrEmail")),
  }), [t]);

  // Login form
  const loginForm = useForm<z.infer<typeof loginSchema>>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      identifier: '',
      password: '',
    },
  });

  // Reset password form
  const resetForm = useForm<z.infer<typeof resetSchema>>({
    resolver: zodResolver(resetSchema),
    defaultValues: {
      identifier: '',
    },
  });

  // Handle login submission
  const onLoginSubmit = async (values: z.infer<typeof loginSchema>) => {
    try {
      const loginResponse = await loginMutation.mutateAsync(values);
      const meResponse = await api.get<Envelope<UserProfileResponse>>('/users/me', {
        headers: {
          Authorization: `Bearer ${loginResponse.data.access_token}`,
        },
      });
      setUser({
        username: meResponse.data.data.username,
        displayName: meResponse.data.data.display_name,
        profilePictureUUID: meResponse.data.data.profile_picture_uuid ?? '',
        isAdmin: meResponse.data.data.is_admin ?? false,
      });
      toast.success(t("auth.loginSuccess"));
      navigate('/'); // Redirect to home page after successful login
    } catch {
      toast.error(t("auth.loginFailed"));
    }
  };

  const handleTestSignIn = async () => {
    const testCredentials = {
      identifier: 'alice@example.com',
      password: 'password123',
    };

    loginForm.setValue('identifier', testCredentials.identifier);
    loginForm.setValue('password', testCredentials.password);

    // Ensure validation is triggered
    const isValid = await loginForm.trigger();
    if (isValid) {
      loginForm.handleSubmit(onLoginSubmit)();
    }
  };

  // Handle forgot password submission
  const onResetSubmit = async () => {
    try {
      // Add your password reset API call here
      // For now, just show a success message
      toast.success(t("auth.resetSent"));
      setMode("login");
    } catch {
      toast.error(t("auth.resetFailed"));
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <Card className="w-full max-w-md">
        {mode === "login" ? (
          <>
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl font-bold tracking-tight">{t("auth.signInTitle")}</CardTitle>
              <CardDescription>{t("auth.signInDescription")}</CardDescription>
            </CardHeader>

            <CardContent>
              <Form {...loginForm}>
                <form onSubmit={loginForm.handleSubmit(onLoginSubmit)} className="space-y-4">
                  <FormField
                    control={loginForm.control}
                    name="identifier"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("auth.usernameOrEmail")}</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              {field.value.includes('@') ? <Mail size={18} /> : <User size={18} />}
                            </div>
                            <Input
                              placeholder={t("auth.usernameOrEmailPlaceholder")}
                              className="pl-10"
                              {...field}
                            />
                          </div>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={loginForm.control}
                    name="password"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("auth.password")}</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              <Lock size={18} />
                            </div>
                            <Input
                              type={showPassword ? "text" : "password"}
                              placeholder={t("auth.passwordPlaceholder")}
                              className="pl-10 pr-10"
                              {...field}
                            />
                            <button
                              type="button"
                              className="absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground hover:text-foreground"
                              onClick={() => setShowPassword(!showPassword)}
                            >
                              {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                            </button>
                          </div>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-2">
                      <input
                        id="remember-me"
                        name="remember-me"
                        type="checkbox"
                        className="w-4 h-4 bg-background text-primary border-input rounded focus:ring-ring"
                      />
                      <label htmlFor="remember-me" className="text-sm text-foreground">
                        {t("auth.rememberMe")}
                      </label>
                    </div>

                    <button
                      type="button"
                      className="text-sm font-medium text-primary hover:text-primary/80"
                      onClick={() => setMode("forgotPassword")}
                    >
                      {t("auth.forgotPassword")}
                    </button>
                  </div>

                  <Button
                    type="submit"
                    className="w-full"
                    disabled={loginMutation.isPending}
                  >
                    {loginMutation.isPending ? t("auth.loggingIn") : t("auth.signIn")}
                  </Button>

                  <Button
                    type="button"
                    variant="secondary"
                    className="w-full"
                    disabled={loginMutation.isPending}
                    onClick={handleTestSignIn}
                  >
                    {loginMutation.isPending ? t("auth.loggingIn") : t("auth.testSignIn")}
                  </Button>


                  <div className="text-center">
                    <p className="text-sm text-muted-foreground">
                      {t("auth.noAccount")}{" "}
                      <Link to="/signup" className="font-medium text-primary hover:text-primary/80">
                        {t("auth.signUp")}
                      </Link>
                    </p>
                  </div>
                </form>
              </Form>
            </CardContent>
          </>
        ) : (
          <>
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl font-bold tracking-tight">{t("auth.resetTitle")}</CardTitle>
              <CardDescription>
                {t("auth.resetDescription")}
              </CardDescription>
            </CardHeader>

            <CardContent>
              <Form {...resetForm}>
                <form onSubmit={resetForm.handleSubmit(onResetSubmit)} className="space-y-4">
                  <FormField
                    control={resetForm.control}
                    name="identifier"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("auth.usernameOrEmail")}</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              {field.value.includes('@') ? <Mail size={18} /> : <User size={18} />}
                            </div>
                            <Input
                              placeholder={t("auth.usernameOrEmailPlaceholder")}
                              className="pl-10"
                              {...field}
                            />
                          </div>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <Button type="submit" className="w-full">
                    {t("auth.sendResetLink")}
                  </Button>

                  <div className="text-center">
                    <button
                      type="button"
                      className="text-sm font-medium text-primary hover:text-primary/80"
                      onClick={() => setMode("login")}
                    >
                      {t("auth.backToLogin")}
                    </button>
                  </div>
                </form>
              </Form>
            </CardContent>
          </>
        )}
      </Card>
    </div>
  );
}

export default LoginPage;