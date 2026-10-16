import { UserRoleForm } from "@/components/admin/user-role-form";
import { Link } from "@/components/ui/link";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminUsers } from "@/lib/data/admin-users";
import { getI18n } from "@/lib/i18n/server";

const PAGE_SIZE = 20;

type Props = {
  params: Promise<{ locale: string }>;
  searchParams: Promise<{ page?: string }>;
};

function href(page: number) {
  return page > 1 ? `/admin/users?page=${page}` : "/admin/users";
}

export default async function AdminUsersPage({ params, searchParams }: Props) {
  const { locale, d } = await getI18n(params);
  const { user: me, token } = await requireAdmin(locale, `/${locale}/admin/users`);
  const sp = await searchParams;
  const page = Math.max(1, Math.floor(Number(sp.page)) || 1);
  const u = d.admin.users;

  // Failures bubble up to the route's error boundary (error.tsx).
  const result = await getAdminUsers(token, page, PAGE_SIZE);
  const pages = Math.max(1, Math.ceil(result.total / PAGE_SIZE));
  const dateFmt = new Intl.DateTimeFormat(locale === "tr" ? "tr-TR" : "en-GB", { day: "numeric", month: "short", year: "numeric" });

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h2 className="headline-md">{u.title}</h2>
        <p className="text-umber">{u.total(result.total)}</p>
      </div>

      {result.items.length === 0 ? (
        <p className="border border-travertine bg-paper p-6 text-umber">{u.empty}</p>
      ) : (
        <div className="overflow-x-auto border border-travertine">
          <table className="w-full min-w-[720px] text-left text-[14px]">
            <thead className="bg-paper">
              <tr className="label-caps text-[10px] text-umber">
                <th scope="col" className="px-4 py-3">{u.columns.name}</th>
                <th scope="col" className="px-4 py-3">{u.columns.email}</th>
                <th scope="col" className="px-4 py-3">{u.columns.role}</th>
                <th scope="col" className="px-4 py-3">{u.columns.joined}</th>
                <th scope="col" className="px-4 py-3 text-right">{u.columns.actions}</th>
              </tr>
            </thead>
            <tbody>
              {result.items.map((user) => (
                <tr key={user.id} className="border-t border-travertine align-top">
                  <td className="px-4 py-3">
                    {`${user.first_name} ${user.last_name}`.trim()}
                    {user.id === me.id ? <span className="ml-2 text-[12px] text-umber">({u.you})</span> : null}
                  </td>
                  <td className="break-all px-4 py-3">{user.email}</td>
                  <td className="px-4 py-3">{u.roles[user.role]}</td>
                  <td className="px-4 py-3">{dateFmt.format(new Date(user.created_at))}</td>
                  <td className="px-4 py-3 text-right">
                    <UserRoleForm userId={user.id} role={user.role} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {pages > 1 ? (
        <div className="flex items-center justify-between gap-4">
          {page > 1 ? <Link href={href(page - 1)} className="btn btn-secondary">{u.prev}</Link> : <span />}
          <span className="text-umber">{u.pageOf(page, pages)}</span>
          {page < pages ? <Link href={href(page + 1)} className="btn btn-secondary">{u.next}</Link> : <span />}
        </div>
      ) : null}
    </div>
  );
}
