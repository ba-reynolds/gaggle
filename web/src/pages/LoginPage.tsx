import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useUser } from '@/contexts/UserContext';
import { useLoginMutation } from '@/hooks/useAuth';
import api from '@/lib/api';
import type { Envelope, UserProfileResponse } from '@/types/api';
import { zodResolver } from '@hookform/resolvers/zod';
import { Eye, EyeOff, Lock, Mail, User } from "lucide-react";
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { Link, useNavigate } from 'react-router-dom';
import { toast } from "sonner";
import * as z from 'zod';

const loginSchema = z.object({
  identifier: z.string()
    .min(1, 'Username or email is required')
    .min(3, 'Username or email must be at least 3 characters long')
    .max(90, 'Username or email must be at most 90 characters long'),
  password: z.string()
    .min(1, 'Password is required')
    .min(8, 'Password must be at least 8 characters long')
    .max(64, 'Password must be at most 64 characters long'),
});

// Form schema for password reset
const resetSchema = z.object({
  identifier: z.string().min(1, 'Username or email is required'),
});

// Main Login Component
const LoginPage: React.FC = () => {
  const [showPassword, setShowPassword] = useState(false);
  const [mode, setMode] = useState("login"); // login or forgotPassword
  const navigate = useNavigate();
  const { setUser } = useUser();
  // Login mutation
  const loginMutation = useLoginMutation();

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
      toast.success("Login successful");
      navigate('/'); // Redirect to home page after successful login
    } catch {
      toast.error("Login failed, invalid credentials");
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
      toast.success("Password reset email sent");
      setMode("login");
    } catch {
      toast.error("Reset request failed, please try again later");
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <Card className="w-full max-w-md">
        {mode === "login" ? (
          <>
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl font-bold tracking-tight">Sign in to your account</CardTitle>
              <CardDescription>Enter your details below to sign in</CardDescription>
            </CardHeader>

            <CardContent>
              <Form {...loginForm}>
                <form onSubmit={loginForm.handleSubmit(onLoginSubmit)} className="space-y-4">
                  <FormField
                    control={loginForm.control}
                    name="identifier"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Username or Email</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              {field.value.includes('@') ? <Mail size={18} /> : <User size={18} />}
                            </div>
                            <Input
                              placeholder="Enter your username or email"
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
                        <FormLabel>Password</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              <Lock size={18} />
                            </div>
                            <Input
                              type={showPassword ? "text" : "password"}
                              placeholder="Enter your password"
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
                        Remember me
                      </label>
                    </div>

                    <button
                      type="button"
                      className="text-sm font-medium text-primary hover:text-primary/80"
                      onClick={() => setMode("forgotPassword")}
                    >
                      Forgot your password?
                    </button>
                  </div>

                  <Button
                    type="submit"
                    className="w-full"
                    disabled={loginMutation.isPending}
                  >
                    {loginMutation.isPending ? "Logging in..." : "Sign in"}
                  </Button>

                  <Button
                    type="button"
                    variant="secondary"
                    className="w-full"
                    disabled={loginMutation.isPending}
                    onClick={handleTestSignIn}
                  >
                    {loginMutation.isPending ? "Logging in..." : "Test sign in"}
                  </Button>


                  <div className="text-center">
                    <p className="text-sm text-muted-foreground">
                      Don't have an account?{" "}
                      <Link to="/signup" className="font-medium text-primary hover:text-primary/80">
                        Sign up
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
              <CardTitle className="text-2xl font-bold tracking-tight">Reset your password</CardTitle>
              <CardDescription>
                Enter your email or username and we'll send you a reset link
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
                        <FormLabel>Username or Email</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              {field.value.includes('@') ? <Mail size={18} /> : <User size={18} />}
                            </div>
                            <Input
                              placeholder="Enter your username or email"
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
                    Send reset link
                  </Button>

                  <div className="text-center">
                    <button
                      type="button"
                      className="text-sm font-medium text-primary hover:text-primary/80"
                      onClick={() => setMode("login")}
                    >
                      Back to login
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