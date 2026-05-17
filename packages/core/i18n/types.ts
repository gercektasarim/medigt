export type Locale = "tr" | "en";

export type Dictionary = {
  common: {
    save: string;
    cancel: string;
    delete: string;
    edit: string;
    create: string;
    search: string;
    loading: string;
    empty: string;
    error: string;
    yes: string;
    no: string;
    confirm: string;
  };
  nav: {
    inbox: string;
    today: string;
    calendar: string;
    settings: string;
    logout: string;
  };
  modules: {
    hasta: string;
    randevu: string;
    poliklinik: string;
    yatis: string;
    laboratuvar: string;
    radyoloji: string;
    ameliyat: string;
    diyaliz: string;
    dis: string;
    hemsire: string;
    ecza: string;
    ilac: string;
    depo: string;
    vezne: string;
    fatura: string;
    hakedis: string;
    kasaRapor: string;
    rapor: string;
    medula: string;
    mernis: string;
    hizmet: string;
    icd10: string;
    personel: string;
    doktor: string;
    brans: string;
    kurum: string;
    ayarlar: string;
    yetki: string;
  };
  auth: {
    login: string;
    email: string;
    password: string;
    code: string;
    sendCode: string;
    verifyCode: string;
    welcome: string;
  };
  hospital: {
    organization: string;
    branch: string;
    department: string;
    newOrganization: string;
    newBranch: string;
    switchHospital: string;
  };
};
