import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useUser } from '@/contexts/UserContext';
import { useI18n } from '@/contexts/I18nContext';
import { useRegisterMutation } from '@/hooks/useAuth';
import { detectBrowserLanguage } from '@/i18n';
import api from '@/lib/api';
import type { Envelope, UserProfileResponse } from '@/types/api';
import { type AxiosError } from 'axios';
import { zodResolver } from '@hookform/resolvers/zod';
import { AtSign, Lock, Mail, User } from "lucide-react";
import { useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { Link, useNavigate } from 'react-router-dom';
import { toast } from "sonner";
import * as z from 'zod';
import GoogleSignInButton from '@/components/GoogleSignInButton';

const SignupPage: React.FC = () => {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { setUser } = useUser();
  const registerMutation = useRegisterMutation();

  const signupSchema = useMemo(() => z.object({
    username: z.string()
      .min(3, t("auth.usernameAtLeast3"))
      .max(16, t("auth.usernameAtMost16"))
      .regex(/^[a-zA-Z0-9_]+$/, t("auth.usernameCharset")),
    email: z.string()
      .min(1, t("auth.emailRequired"))
      .email(t("auth.emailInvalid")),
    password: z.string()
      .min(8, t("auth.passwordAtLeast8"))
      .max(72, t("auth.passwordAtMost72")),
  }), [t]);

  const signupForm = useForm<z.infer<typeof signupSchema>>({
    resolver: zodResolver(signupSchema),
    defaultValues: {
      username: '',
      email: '',
      password: '',
    },
  });

  const onSignupSubmit = async (values: z.infer<typeof signupSchema>) => {
    try {
      const registerResponse = await registerMutation.mutateAsync({
        ...values,
        language: detectBrowserLanguage(),
      });
      const meResponse = await api.get<Envelope<UserProfileResponse>>('/users/me', {
        headers: {
          Authorization: `Bearer ${registerResponse.data.access_token}`,
        },
      });
      setUser({
        username: meResponse.data.data.username,
        displayName: meResponse.data.data.display_name,
        profilePictureUUID: meResponse.data.data.profile_picture_uuid ?? '',
        isAdmin: meResponse.data.data.is_admin ?? false,
      });
      toast.success(t("auth.signupSuccess"));
      navigate('/'); // Redirect to home page after successful signup
    } catch (err) {
      const apiMessage = (err as AxiosError<Envelope<unknown>> | undefined)?.response?.data?.error?.message;
      toast.error(apiMessage ?? t("auth.signupFailed"));
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold tracking-tight">{t("auth.createAccountTitle")}</CardTitle>
          <CardDescription>{t("auth.createAccountDescription")}</CardDescription>
        </CardHeader>

        <CardContent>
          <Form {...signupForm}>
            <form onSubmit={signupForm.handleSubmit(onSignupSubmit)} className="space-y-4">
              <FormField
                control={signupForm.control}
                name="username"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("auth.username")}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                          <AtSign size={18} />
                        </div>
                        <Input
                          placeholder={t("auth.usernamePlaceholder")}
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
                control={signupForm.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("auth.email")}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                          <Mail size={18} />
                        </div>
                        <Input
                          type="email"
                          placeholder={t("auth.emailPlaceholder")}
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
                control={signupForm.control}
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
                          type="password"
                          placeholder={t("auth.passwordHint")}
                          className="pl-10"
                          {...field}
                        />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                <User className="h-4 w-4" />
                <span>{t("auth.terms")}</span>
              </div>

              <Button
                type="submit"
                className="w-full"
                disabled={registerMutation.isPending}
              >
                {registerMutation.isPending ? t("auth.creatingAccount") : t("auth.signUp")}
              </Button>

              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-card px-2 text-muted-foreground">Or</span>
                </div>
              </div>

              <GoogleSignInButton />

              <div className="text-center">
                <p className="text-sm text-muted-foreground">
                  {t("auth.alreadyHaveAccount")}{" "}
                  <Link to="/login" className="font-medium text-primary hover:text-primary/80">
                    {t("auth.signIn")}
                  </Link>
                </p>
              </div>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}

export default SignupPage;