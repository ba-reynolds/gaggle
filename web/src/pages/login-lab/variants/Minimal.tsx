import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from '@/components/ui/form';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useLoginFlow } from '@/hooks/useLoginFlow';
import { Link } from 'react-router-dom';

export function Minimal() {
  const { form, loginMutation, onSubmit } = useLoginFlow();

  return (
    <div className="flex h-full w-full items-center justify-center p-8">
      <div className="w-full max-w-sm space-y-10">
        <div className="space-y-2">
          <span className="text-sm uppercase tracking-[0.25em] text-muted-foreground">
            Gaggle
          </span>
          <h1 className="text-4xl font-light tracking-tight">Sign in.</h1>
        </div>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-8">
            <div className="space-y-2">
              <FormField
                control={form.control}
                name="identifier"
                render={({ field }) => (
                  <FormItem>
                    <FormControl>
                      <Input
                        placeholder="Username or email"
                        className="border-0 border-b bg-transparent px-0 py-2 shadow-none focus-visible:ring-0 rounded-none"
                        {...field}
                      />
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
                    <FormControl>
                      <Input
                        type="password"
                        placeholder="Password"
                        className="border-0 border-b bg-transparent px-0 py-2 shadow-none focus-visible:ring-0 rounded-none"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <Button
              type="submit"
              variant="outline"
              className="w-full rounded-full"
              disabled={loginMutation.isPending}
            >
              {loginMutation.isPending ? 'Signing in...' : 'Sign in'}
            </Button>
          </form>
        </Form>

        <p className="text-sm text-muted-foreground">
          Don&apos;t have an account?{" "}
          <Link to="/signup" className="text-foreground underline underline-offset-4">
            Create one
          </Link>
        </p>
      </div>
    </div>
  );
}