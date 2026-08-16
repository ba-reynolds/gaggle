import type { ReactNode } from 'react';
import { createContext, useContext, useState } from 'react';


export interface User {
  username: string;
  displayName: string;
  profilePictureUUID: string;
}

interface UserContextType {
  user: User;
  setUser: (userData: Partial<User>) => void;
}

const UserContext = createContext<UserContextType | undefined>(undefined);

interface UserProviderProps {
  children: ReactNode;
}

export function UserProvider({ children }: UserProviderProps) {
  // Use localStorage to persist user data
  const [user, setUser] = useState<User>({
    displayName: "",
    username: "",
    profilePictureUUID: ""
  });

  const updateUser = (userData: Partial<User>) => {
    setUser({ ...user, ...userData });
  };

  return (
    <UserContext.Provider value={{ user, setUser: updateUser }}>
      {children}
    </UserContext.Provider>
  );
}

export function useUser() {
  const context = useContext(UserContext);
  if (context === undefined) {
    throw new Error('useUser must be used within a UserProvider');
  }
  return context;
} 