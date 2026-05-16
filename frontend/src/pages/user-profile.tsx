import { UserProfileView } from "@/pages/profile";

export default function UserProfilePage({ params }: { params: { id: string } }) {
  return <UserProfileView userId={params.id} backHref="/" />;
}
