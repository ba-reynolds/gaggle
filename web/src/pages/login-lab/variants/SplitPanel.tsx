import { ArrowRight, Eye, EyeOff, Lock, Mail, User } from 'lucide-react';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useLoginFlow } from '@/hooks/useLoginFlow';
import { useState } from 'react';
import { Link } from 'react-router-dom';

export function SplitPanel() {
  const { form, loginMutation, onSubmit } = useLoginFlow();
  const [showPassword, setShowPassword] = useState(false);

  return (
    <div className="flex h-full w-full">
      <div className="hidden flex-1 flex-col justify-between bg-gradient-to-br from-primary via-primary to-chart-1 p-12 text-primary-foreground lg:flex">
        <div className="flex items-center gap-3">
          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-background/20 text-xl font-bold">
            G
          </span>
          <span className="text-lg font-semibold tracking-tight">Gaggle</span>
        </div>
        <div className="max-w-md space-y-6">
          <h2 className="text-4xl font-bold leading-tight tracking-tight">
            A social feed that feels like home.
          </h2>
          <p className="text-primary-foreground/80">
            Follow your people. Join the flock. Say the thing out loud.
          </p>
        </div>
        <ul className="space-y-3 text-sm text-primary-foreground/90">
          <li className="flex items-center gap-2">
            <ArrowRight size={16} />
            One feed for everyone you follow
          </li>
          <li className="flex items-center gap-2">
            <ArrowRight size={16} />
            Real conversations, zero noise
          </li>
          <li className="flex items-center gap-2">
            <ArrowRight size={16} />
            Honk responsibly
          </li>
        </ul>
      </div>

      <div className="flex flex-1 items-center justify-center p-8">
        <div className="w-full max-w-sm space-y-8">
          <div className="lg:hidden">
            <span className="flex h-12 w-12 items-center justify-center rounded-full bg-primary text-xl font-bold text-primary-foreground">
              G
            </span>
          </div>

          <div className="space-y-1">
            <h1 className="text-2xl font-bold tracking-tight">Welcome back</h1>
            <p className="text-sm text-muted-foreground">Sign in to continue to Gaggle</p>
          </div>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <FormField
                control={form.control}
                name="identifier"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Username or Email</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                          {field.value.includes('@') ? <Mail size={18} /> : <User size={18} />}
                        </div>
                        <Input placeholder="you@example.com" className="pl-10" {...field} />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem>
                    <div className="flex items-center justify-between">
                      <FormLabel>Password</FormLabel>
                      <Link to="/login" className="text-xs text-muted-foreground hover:text-foreground">
                        Forgot password?
                      </Link>
                    </div>
                    <FormControl>
                      <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                          <Lock size={18} />
                        </div>
                        <Input
                          type={showPassword ? 'text' : 'password'}
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

              <Button type="submit" className="w-full" disabled={loginMutation.isPending}>
                {loginMutation.isPending ? 'Signing in...' : 'Sign in'}
              </Button>
            </form>
          </Form>
        </div>
      </div>
    </div>
  );
}