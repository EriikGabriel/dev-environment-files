import { useState } from "react";

interface Props {
  onSubmit: (password: string) => void;
  onCancel: () => void;
}

/** Asks for the sudo password. It is sent to the sidecar's stdin, never stored. */
export function PasswordDialog({ onSubmit, onCancel }: Props) {
  const [pw, setPw] = useState("");
  return (
    <div className="overlay" role="dialog" aria-modal="true" aria-label="Senha de administrador">
      <form
        className="dialog"
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit(pw);
        }}
      >
        <h2>Senha de administrador</h2>
        <p className="muted">
          Instalar pacotes precisa de <code>sudo</code>. A senha é passada ao instalador só nesta execução e não é
          salva. Deixe em branco se o sudo não pedir senha.
        </p>
        <p className="muted small-note">
          A senha fica só na memória do instalador durante esta execução; não vai para o log nem para o disco.
        </p>
        <input type="password" autoFocus value={pw} onChange={(e) => setPw(e.target.value)} placeholder="senha do sudo" />
        <div className="row end">
          <button type="button" className="ghost" onClick={onCancel}>
            Cancelar
          </button>
          <button type="submit" className="primary">
            Continuar
          </button>
        </div>
      </form>
    </div>
  );
}
