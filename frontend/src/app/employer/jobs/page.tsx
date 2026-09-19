import { redirect } from "next/navigation";

export default function EmployerJobsPage() {
  redirect("/activity?tab=postings");
}
