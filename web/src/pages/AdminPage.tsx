import { useCallback, useMemo, useState } from "react";
import { SEARCH_DEBOUNCE_MS, useDebounce } from "@/hooks/useDebounce";
import { searchUsers } from "@/api/search";
import { useBadgeCatalog, useCreateBadge, useDeleteBadge, useGrantBadge, useRevokeBadge, useUpdateBadge } from "@/hooks/useAdmin";
import { useQuery } from "@tanstack/react-query";
import type { CreateBadgePayload, UserProfileResponse } from "@/types/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { CustomDialogContent } from "@/components/ui/custom-dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Loader2, Plus, Trash2, Pencil } from "lucide-react";
import { toast } from "sonner";
import { useUser } from "@/contexts/UserContext";
import ConfirmDialog from "@/components/ConfirmDialog";

const emptyForm = (): CreateBadgePayload => ({ key: "", label: "", description: "", icon: "Award" });

const AdminPage: React.FC = () => {
  const { user } = useUser();
  const catalogQuery = useBadgeCatalog();
  const createBadge = useCreateBadge();
  const updateBadge = useUpdateBadge();
  const deleteBadge = useDeleteBadge();
  const grantBadge = useGrantBadge();
  const revokeBadge = useRevokeBadge();

  const [userQuery, setUserQuery] = useState("");
  const debouncedQuery = useDebounce(userQuery, SEARCH_DEBOUNCE_MS);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<{ id: number } | null>(null);
  const [form, setForm] = useState<CreateBadgePayload>(emptyForm());
  const [grantedFor, setGrantedFor] = useState<Record<string, Record<number, boolean>>>({});
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);

  const catalog = useMemo(() => catalogQuery.data?.data ?? [], [catalogQuery.data]);

  const { data: resultsData } = useQuery({
    queryKey: ['admin-user-search', debouncedQuery],
    queryFn: () => searchUsers(debouncedQuery),
    enabled: debouncedQuery.length > 0,
    retry: false,
  });
  const results = useMemo(() => resultsData?.data?.items ?? [], [resultsData]);

  const hasBadge = useCallback(
    (username: string, badgeId: number) => {
      const flags = grantedFor[username];
      if (flags && flags[badgeId] != null) return flags[badgeId];
      const profile = results.find(u => u.username === username);
      return profile?.badges?.some(b => b.id === badgeId) ?? false;
    },
    [grantedFor, results]
  );

  const toggleGrant = (username: string, badgeId: number) => {
    setGrantedFor(prev => ({
      ...prev,
      [username]: { ...(prev[username] ?? {}), [badgeId]: !hasBadge(username, badgeId) },
    }));
    if (hasBadge(username, badgeId)) {
      revokeBadge.mutate({ username, badgeId }, { onError: () => toast.error("Failed to revoke badge") });
    } else {
      grantBadge.mutate({ username, badgeId }, { onError: () => toast.error("Failed to grant badge") });
    }
  };

  const openCreate = () => {
    setEditing(null);
    setForm(emptyForm());
    setFormOpen(true);
  };

  const openEdit = (id: number) => {
    const b = catalog.find(x => x.id === id);
    if (!b) return;
    setEditing({ id });
    setForm({ key: b.key, label: b.label, description: b.description, icon: b.icon });
    setFormOpen(true);
  };

  const submitForm = () => {
    if (editing) {
      updateBadge.mutate({ badgeId: editing.id, payload: form }, {
        onSuccess: () => { setFormOpen(false); toast.success("Badge updated"); },
        onError: (e: Error) => toast.error(e.message || "Failed to update badge"),
      });
    } else {
      createBadge.mutate(form, {
        onSuccess: () => { setFormOpen(false); toast.success("Badge created"); },
        onError: (e: Error) => toast.error(e.message || "Failed to create badge"),
      });
    }
  };

  const removeBadge = (id: number) => {
    deleteBadge.mutate(id, {
      onSuccess: () => toast.success("Badge deleted"),
      onError: () => toast.error("Delete failed. Does a user still hold this badge?"),
    });
  };

  const earnedBadges = useMemo(() => catalog.filter(b => b.kind === 'earned'), [catalog]);
  const assignedBadges = useMemo(() => catalog.filter(b => b.kind === 'assigned'), [catalog]);

  if (!user.isAdmin) {
    return <div className="text-center py-20 text-muted-foreground">Admin access required.</div>;
  }

  const renderBadgeRow = (b: { id: number; key: string; label: string; description: string; icon: string; kind: string }) => (
    <div key={b.id} className="flex items-center justify-between rounded-lg border border-border p-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <Badge variant="outline">{b.label}</Badge>
          <span className="text-xs text-muted-foreground">{b.kind}</span>
        </div>
        <p className="text-sm text-primary mt-1 truncate">{b.description}</p>
      </div>
      {b.kind === 'assigned' && (
        <div className="flex gap-1 ml-3">
          <Button variant="ghost" size="icon" onClick={() => openEdit(b.id)}><Pencil className="h-4 w-4" /></Button>
          <Button variant="ghost" size="icon" className="text-destructive" onClick={() => setDeleteTarget(b.id)}><Trash2 className="h-4 w-4" /></Button>
        </div>
      )}
    </div>
  );

  return (
    <div className="w-full max-w-4xl mx-auto px-4 py-6 space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-primary">Admin</h1>
        <Button onClick={openCreate}><Plus className="h-4 w-4 mr-2" />New badge</Button>
      </div>

      <section>
        <h2 className="text-lg font-semibold text-primary mb-3">Badge catalog</h2>
        {catalogQuery.isLoading ? (
          <div className="flex justify-center py-8"><Loader2 className="h-6 w-6 animate-spin" /></div>
        ) : (
          <>
            {earnedBadges.length > 0 && (
              <>
                <h3 className="text-sm font-medium text-muted-foreground mb-2">Earned (auto)</h3>
                <div className="space-y-2 mb-4">{earnedBadges.map(renderBadgeRow)}</div>
              </>
            )}
            {assignedBadges.length > 0 && (
              <>
                <h3 className="text-sm font-medium text-muted-foreground mb-2">Assigned</h3>
                <div className="space-y-2">{assignedBadges.map(renderBadgeRow)}</div>
              </>
            )}
          </>
        )}
      </section>

      <section>
        <h2 className="text-lg font-semibold text-primary mb-3">Assign badges</h2>
        <Input placeholder="Search users by name or username..." value={userQuery} onChange={e => setUserQuery(e.target.value)} />
        {debouncedQuery && (
          <div className="mt-3 space-y-2">
            {results.length === 0 ? (
              <p className="text-sm text-muted-foreground">No users found.</p>
            ) : (
              results.map((profile: UserProfileResponse) => (
                <div key={profile.username} className="rounded-lg border border-border p-3">
                  <p className="font-semibold text-primary">{profile.display_name}</p>
                  <p className="text-xs text-muted-foreground">@{profile.username}</p>
                  <div className="flex flex-wrap gap-1.5 mt-2">
                    {assignedBadges.map(b => {
                      const granted = hasBadge(profile.username, b.id);
                      return (
                        <Button
                          key={b.id}
                          size="sm"
                          variant={granted ? "default" : "outline"}
                          onClick={() => toggleGrant(profile.username, b.id)}
                        >
                          {b.label}
                        </Button>
                      );
                    })}
                    {assignedBadges.length === 0 && <span className="text-xs text-muted-foreground">Create an assigned badge first.</span>}
                  </div>
                </div>
              ))
            )}
          </div>
        )}
      </section>

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <CustomDialogContent className="sm:max-w-md bg-card">
          <DialogHeader><DialogTitle className="text-primary">{editing ? "Edit badge" : "New badge"}</DialogTitle></DialogHeader>
          <div className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="badge-key">Key</Label>
              <Input id="badge-key" value={form.key} onChange={e => setForm({ ...form, key: e.target.value })} placeholder="staff" maxLength={50} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="badge-label">Label</Label>
              <Input id="badge-label" value={form.label} onChange={e => setForm({ ...form, label: e.target.value })} placeholder="Staff" maxLength={60} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="badge-desc">Description</Label>
              <Textarea id="badge-desc" value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} placeholder="Why this badge exists" maxLength={200} className="min-h-20" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="badge-icon">Icon (lucide-react name)</Label>
              <Input id="badge-icon" value={form.icon} onChange={e => setForm({ ...form, icon: e.target.value })} placeholder="Award" maxLength={50} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setFormOpen(false)}>Cancel</Button>
            <Button disabled={!form.key.trim() || !form.label.trim() || !form.description.trim() || (createBadge.isPending || updateBadge.isPending)} onClick={submitForm}>
              {(createBadge.isPending || updateBadge.isPending) && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              Save
            </Button>
          </DialogFooter>
        </CustomDialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete this badge?"
        description="Badges still assigned to users cannot be deleted."
        confirmLabel="Delete"
        onConfirm={() => deleteTarget !== null && removeBadge(deleteTarget)}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      />
    </div>
  );
};

export default AdminPage;