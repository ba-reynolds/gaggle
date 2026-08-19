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
import { Lock, Mail, User } from 'lucide-react';

export function Glassmorphism() {
  const { form, loginMutation, onSubmit } = useLoginFlow();

  return (
    <div className="relative flex h-full w-full items-center justify-center overflow-hidden bg-background p-8">
      <div
        className="pointer-events-none absolute -left-32 -top-32 h-96 w-96 rounded-full bg-chart-1/40 blur-3xl"
        style={{ animation: 'login-lab-drift 18s ease-in-out infinite' }}
      />
      <div
        className="pointer-events-none absolute -bottom-40 -right-20 h-[28rem] w-[28rem] rounded-full bg-chart-5/40 blur-3xl"
        style={{ animation: 'login-lab-drift 22s ease-in-out infinite reverse' }}
      />
      <div
        className="pointer-events-none absolute left-1/2 top-1/2 h-72 w-72 -translate-x-1/2 -translate-y-1/2 rounded-full bg-chart-2/30 blur-3xl"
        style={{ animation: 'login-lab-drift 26s ease-in-out infinite 4s' }}
      />

      <div className="relative w-full max-w-sm rounded-2xl border border-background/60 bg-background/60 p-8 shadow-2xl backdrop-blur-xl">
        <div className="mb-6 space-y-1">
          <div className="flex items-center gap-3">
            <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-gradient-to-br from-chart-1 to-chart-5 text-lg font-bold text-white shadow-lg">
              G
            </span>
            <div>
              <h1 className="text-xl font-bold tracking-tight">Gaggle</h1>
              <p className="text-xs text-muted-foreground">Welcome back to the flock</p>
            </div>
          </div>
        </div>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="identifier"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Username or email</FormLabel>
                  <FormControl>
                    <div className="relative">
                      <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                        {field.value.includes('@') ? <Mail size={18} /> : <User size={18} />}
                      </div>
                      <Input
                        placeholder="you@example.com"
                        className="border-background/80 bg-background/50 pl-10 backdrop-blur"
                        {...field}
                      />
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
                  <FormLabel>Password</FormLabel>
                  <FormControl>
                    <div className="relative">
                      <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                        <Lock size={18} />
                      </div>
                      <Input
                        type="password"
                        placeholder="Enter your password"
                        className="border-background/80 bg-background/50 pl-10 backdrop-blur"
                        {...field}
                      />
                    </div>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="flex items-center justify-end">
              <button type="button" className="text-xs text-muted-foreground hover:text-foreground">
                Forgot password?
              </button>
            </div>

            <Button
              type="submit"
              className="w-full"
              disabled={loginMutation.isPending}
            >
              {loginMutation.isPending ? 'Signing in...' : 'Continue to Gaggle'}
            </Button>
          </form>
        </Form>
      </div>
    </div>
  );
}