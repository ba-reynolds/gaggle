import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useUser } from '@/contexts/UserContext';
import { useRegisterMutation } from '@/hooks/useAuth';
import api from '@/lib/api';
import type { Envelope, UserProfileResponse } from '@/types/api';
import { type AxiosError } from 'axios';
import { zodResolver } from '@hookform/resolvers/zod';
import { AtSign, Lock, Mail, User } from "lucide-react";
import { useForm } from 'react-hook-form';
import { Link, useNavigate } from 'react-router-dom';
import { toast } from "sonner";
import * as z from 'zod';

const signupSchema = z.object({
  username: z.string()
    .min(3, 'Username must be at least 3 characters long')
    .max(16, 'Username must be at most 16 characters long')
    .regex(/^[a-zA-Z0-9_]+$/, 'Username can only contain letters, numbers, and underscores'),
  email: z.string()
    .min(1, 'Email is required')
    .email('Please enter a valid email address'),
  password: z.string()
    .min(8, 'Password must be at least 8 characters long')
    .max(72, 'Password must be at most 72 characters long'),
});

const SignupPage: React.FC = () => {
  const navigate = useNavigate();
  const { setUser } = useUser();
  const registerMutation = useRegisterMutation();

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
      const registerResponse = await registerMutation.mutateAsync(values);
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
      toast.success("Account created successfully");
      navigate('/'); // Redirect to home page after successful signup
    } catch (err) {
      const apiMessage = (err as AxiosError<Envelope<unknown>> | undefined)?.response?.data?.error?.message;
      toast.error(apiMessage ?? "Sign up failed. Please check your details and try again.");
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold tracking-tight">Create your account</CardTitle>
          <CardDescription>Join Gaggle — it only takes a moment</CardDescription>
        </CardHeader>

        <CardContent>
          <Form {...signupForm}>
            <form onSubmit={signupForm.handleSubmit(onSignupSubmit)} className="space-y-4">
              <FormField
                control={signupForm.control}
                name="username"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Username</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                          <AtSign size={18} />
                        </div>
                        <Input
                          placeholder="Choose a username"
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
                    <FormLabel>Email</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                          <Mail size={18} />
                        </div>
                        <Input
                          type="email"
                          placeholder="you@example.com"
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
                    <FormLabel>Password</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                          <Lock size={18} />
                        </div>
                        <Input
                          type="password"
                          placeholder="At least 8 characters"
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
                <span>By signing up, you agree to our Terms of Service.</span>
              </div>

              <Button
                type="submit"
                className="w-full"
                disabled={registerMutation.isPending}
              >
                {registerMutation.isPending ? "Creating account..." : "Sign up"}
              </Button>

              <div className="text-center">
                <p className="text-sm text-muted-foreground">
                  Already have an account?{" "}
                  <Link to="/login" className="font-medium text-primary hover:text-primary/80">
                    Sign in
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