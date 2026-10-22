import { inject, ref, type InjectionKey, type Ref } from 'vue'

export const userPrivilegeTitleKey: InjectionKey<Readonly<Ref<string | null>>> = Symbol('user-privilege-title')

export function useUserPrivilegeTitle() {
  return inject(userPrivilegeTitleKey, ref(null))
}
