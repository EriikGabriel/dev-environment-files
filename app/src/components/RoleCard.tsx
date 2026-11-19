import { UserRound, Users } from "lucide-react";
import { useEffect, useState } from "react";
import * as api from "../api";
import { errorMessage } from "../api";
import type { Role } from "../types";
import { Help } from "./Help";

/** Whose configuration this is: the owner gets everything back; someone else gets the programs and settings, without the owner's saves and accounts. */
export function RoleCard({ root, onChanged }: { root: string; onChanged?: () => void }) {
  const [role, setRole] = useState<Role | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.role(root).then(setRole, (e) => setError(errorMessage(e)));
  }, [root]);

  if (!role) return error ? <p className="banner bad">{error}</p> : null;

  const pick = async (owner: boolean) => {
    if (owner === role.owner && role.explicit) return;
    setBusy(true);
    try {
      setRole(await api.setRole(root, owner));
      onChanged?.();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card role-card">
      <div className="grow-col">
        <strong>
          De quem é esta configuração?
          <Help title="Dono ou outra pessoa">
            O app vem com as configurações do dono do repositório: programas, drivers, Windhawk, Playnite (temas, extensões e ajustes) e
            RetroArch. Quem não é o dono recebe tudo isso, menos o que é pessoal: os saves, estados e playlists do RetroArch, o histórico de
            jogo e as contas (Steam ID, usuário do RetroAchievements). Os periféricos e o driver de vídeo vêm marcados só se o aparelho ou a
            placa estiverem nesta máquina. Ao salvar a sua própria configuração, ela passa a ser sua.
          </Help>
        </strong>
        <span className="muted small-note">
          {role.explicit ? "Escolhido nesta máquina." : role.clone ? "Palpite: a pasta de dados é um clone do Git." : "Palpite: a pasta de dados veio do instalador."}
          {!role.owner && " Saves, histórico e contas do dono ficam de fora."}
        </span>
        {error && <span className="banner bad">{error}</span>}
      </div>
      <div className="segmented" role="radiogroup" aria-label="De quem é a configuração">
        <button role="radio" aria-checked={role.owner} className={role.owner ? "active" : ""} disabled={busy} onClick={() => pick(true)}>
          <UserRound size={15} /> É minha
        </button>
        <button role="radio" aria-checked={!role.owner} className={!role.owner ? "active" : ""} disabled={busy} onClick={() => pick(false)}>
          <Users size={15} /> De outra pessoa
        </button>
      </div>
    </div>
  );
}
