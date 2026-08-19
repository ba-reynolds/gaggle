import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useLoginFlow } from '@/hooks/useLoginFlow';
import { StepFlow } from '@/pages/login-lab/variants/StepFlow';
import { zodResolver } from '@hookform/resolvers/zod';
import { Mail, User } from "lucide-react";
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { Link } from 'react-router-dom';
import { toast } from "sonner";
import * as z from 'zod';

// Form schema for password reset
const resetSchema = z.object({
  identifier: z.string().min(1, 'Username or email is required'),
});

// Main Login Component
const LoginPage: React.FC = () => {
  const [mode, setMode] = useState("login"); // login or forgotPassword
  const { form: loginForm, loginMutation, onSubmit: onLoginSubmit } = useLoginFlow();

  // Reset password form
  const resetForm = useForm<z.infer<typeof resetSchema>>({
    resolver: zodResolver(resetSchema),
    defaultValues: {
      identifier: '',
    },
  });

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

  if (mode === "forgotPassword") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <Card className="w-full max-w-md">
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
        </Card>
      </div>
    );
  }

  return (
    <div className="flex h-screen overflow-y-auto bg-background">
      <StepFlow
        footer={
          <div className="space-y-3 border-t pt-6">
            <Button
              type="button"
              variant="secondary"
              className="w-full"
              disabled={loginMutation.isPending}
              onClick={handleTestSignIn}
            >
              {loginMutation.isPending ? "Logging in..." : "Test sign in"}
            </Button>

            <Button
              type="button"
              variant="ghost"
              className="w-full text-muted-foreground"
              onClick={() => setMode("forgotPassword")}
            >
              Forgot your password?
            </Button>

            <p className="text-center text-sm text-muted-foreground">
              Don't have an account?{" "}
              <Link to="/signup" className="font-medium text-primary hover:text-primary/80">
                Sign up
              </Link>
            </p>
          </div>
        }
      />
    </div>
  );
}

export default LoginPage;