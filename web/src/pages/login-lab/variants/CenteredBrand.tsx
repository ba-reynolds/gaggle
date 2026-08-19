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

export function CenteredBrand() {
  const { form, loginMutation, onSubmit } = useLoginFlow();

  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-10 p-8">
      <div className="flex flex-col items-center gap-4 text-center">
        <span className="flex h-16 w-16 items-center justify-center rounded-full bg-primary text-3xl font-black text-primary-foreground shadow-lg">
          G
        </span>
        <div className="space-y-1">
          <h1 className="text-3xl font-black tracking-tight">gaggle</h1>
          <p className="text-sm text-muted-foreground">Get back to the flock</p>
        </div>
      </div>

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="w-full max-w-xs space-y-3">
          <FormField
            control={form.control}
            name="identifier"
            render={({ field }) => (
              <FormItem>
                <FormControl>
                  <Input placeholder="Username or email" className="h-12 text-center" {...field} />
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
                    className="h-12 text-center"
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button
            type="submit"
            className="h-12 w-full text-base font-semibold"
            disabled={loginMutation.isPending}
          >
            {loginMutation.isPending ? 'Signing in...' : 'Sign in'}
          </Button>
        </form>
      </Form>
    </div>
  );
}